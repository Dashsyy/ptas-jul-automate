// Package service holds all business logic for the rental billing system:
// rooms, billing periods, meter readings, payment status, and invoice
// generation. It has no knowledge of Telegram or any other delivery
// mechanism — adapters (internal/telegram today, potentially a CLI or HTTP
// API later) call into it and translate its plain Go types into whatever
// their transport needs.
package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ptas-bot/internal/billing"
	"ptas-bot/internal/models"
	"ptas-bot/internal/store"
)

var ErrNoActivePeriod = errors.New("no active billing period — run NewMonth first")
var ErrNoPendingReading = errors.New("no reading in progress for this chat")

type Service struct {
	rooms   *store.RoomStore
	periods *store.PeriodStore
	bills   *store.BillStore
	pending *store.PendingStore
	rates   billing.Rates
	loc     *time.Location
}

func New(db *sql.DB, rates billing.Rates, loc *time.Location) *Service {
	return &Service{
		rooms:   store.NewRoomStore(db),
		periods: store.NewPeriodStore(db),
		bills:   store.NewBillStore(db),
		pending: store.NewPendingStore(db),
		rates:   rates,
		loc:     loc,
	}
}

func (s *Service) ListRooms() ([]models.Room, error) {
	return s.rooms.List()
}

func (s *Service) SetTenantName(roomNumber int, name string) error {
	return s.rooms.SetTenantName(roomNumber, name)
}

// NewMonth opens a new billing period labeled e.g. "2026-09" and creates a
// blank bill for every room, carrying each room's last "current" meter
// reading forward as this period's "previous" reading.
func (s *Service) NewMonth(label string) (int, error) {
	if _, err := s.periods.GetByLabel(label); err == nil {
		return 0, fmt.Errorf("period %s already exists", label)
	} else if err != sql.ErrNoRows {
		return 0, err
	}

	period, err := s.periods.Create(label, time.Now().In(s.loc))
	if err != nil {
		return 0, err
	}

	rooms, err := s.rooms.List()
	if err != nil {
		return 0, err
	}

	for _, room := range rooms {
		water, elec, err := s.bills.LastReadingsForRoom(room.ID)
		if err != nil {
			return 0, err
		}
		if err := s.bills.EnsureForPeriod(room, period.ID, water, elec); err != nil {
			return 0, err
		}
	}

	return len(rooms), nil
}

func (s *Service) CurrentPeriod() (models.BillingPeriod, error) {
	p, err := s.periods.Current()
	if err == sql.ErrNoRows {
		return p, ErrNoActivePeriod
	}
	return p, err
}

// UnpaidBills lists every bill in the current period that isn't marked paid
// or no-charge, ordered by room number.
func (s *Service) UnpaidBills() ([]models.Bill, error) {
	period, err := s.CurrentPeriod()
	if err != nil {
		return nil, err
	}
	all, err := s.bills.ListForPeriod(period.ID)
	if err != nil {
		return nil, err
	}

	var unpaid []models.Bill
	for _, b := range all {
		if b.Status == models.BillStatusUnpaid {
			unpaid = append(unpaid, b)
		}
	}
	return unpaid, nil
}

// RoomsMissingReadings lists bills in the current period that haven't had
// meter readings entered yet.
func (s *Service) RoomsMissingReadings() ([]models.Bill, error) {
	period, err := s.CurrentPeriod()
	if err != nil {
		return nil, err
	}
	all, err := s.bills.ListForPeriod(period.ID)
	if err != nil {
		return nil, err
	}

	var missing []models.Bill
	for _, b := range all {
		if b.WaterCurr == nil {
			missing = append(missing, b)
		}
	}
	return missing, nil
}

func (s *Service) MarkPaid(billID int64) (models.Bill, error) {
	if err := s.bills.MarkPaid(billID); err != nil {
		return models.Bill{}, err
	}
	return s.bills.GetByID(billID)
}

func (s *Service) GetBill(billID int64) (models.Bill, error) {
	return s.bills.GetByID(billID)
}

// StartReadingEntry begins the two-step (water, then electricity) reading
// conversation for a chat and returns the first prompt to show the user.
func (s *Service) StartReadingEntry(chatID int64, roomNumber int) (string, error) {
	room, err := s.rooms.GetByNumber(roomNumber)
	if err != nil {
		return "", err
	}
	period, err := s.CurrentPeriod()
	if err != nil {
		return "", err
	}
	bill, err := s.bills.GetByRoomPeriod(room.ID, period.ID)
	if err != nil {
		return "", err
	}

	err = s.pending.Set(models.PendingAction{
		ChatID:  chatID,
		Kind:    "enter_readings",
		BillID:  bill.ID,
		Step:    "await_water",
		Payload: "{}",
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("💧 Room %d — enter the WATER meter reading (previous: %s m³):",
		room.Number, trimFloat(bill.WaterPrev)), nil
}

type readingPayload struct {
	Water float64 `json:"water"`
}

// SubmitReadingValue advances the pending reading-entry conversation for a
// chat. It returns done=true and the rendered invoice text once both
// readings have been collected and the bill has been computed and saved.
func (s *Service) SubmitReadingValue(chatID int64, value float64) (done bool, message string, err error) {
	pa, err := s.pending.Get(chatID)
	if err == sql.ErrNoRows {
		return false, "", ErrNoPendingReading
	}
	if err != nil {
		return false, "", err
	}

	bill, err := s.bills.GetByID(pa.BillID)
	if err != nil {
		return false, "", err
	}

	switch pa.Step {
	case "await_water":
		payload, err := json.Marshal(readingPayload{Water: value})
		if err != nil {
			return false, "", err
		}
		pa.Step = "await_elec"
		pa.Payload = string(payload)
		if err := s.pending.Set(pa); err != nil {
			return false, "", err
		}
		return false, fmt.Sprintf("⚡ Room %d — enter the ELECTRICITY meter reading (previous: %s kWh):",
			bill.RoomNumber, trimFloat(bill.ElecPrev)), nil

	case "await_elec":
		var stored readingPayload
		if err := json.Unmarshal([]byte(pa.Payload), &stored); err != nil {
			return false, "", err
		}

		period, err := s.periods.GetByID(bill.PeriodID)
		if err != nil {
			return false, "", err
		}
		// Use the period's own month (parsed from its "YYYY-MM" label) for the
		// days-in-month math, not the current wall-clock date — a bill for
		// August must prorate against August's day count even if it's
		// actually being entered in September.
		periodMonth, err := time.ParseInLocation("2006-01", period.Label, s.loc)
		if err != nil {
			periodMonth = period.StartedAt.In(s.loc)
		}
		daysInMonth := billing.DaysInMonth(periodMonth)

		computed := billing.Compute(billing.ReadingsInput{
			RentUSD:     bill.RentUSD,
			DaysStayed:  bill.DaysStayed,
			DaysInMonth: daysInMonth,
			WaterPrev:   bill.WaterPrev,
			WaterCurr:   stored.Water,
			ElecPrev:    bill.ElecPrev,
			ElecCurr:    value,
		}, s.rates)

		if err := s.bills.SaveReadings(bill.ID,
			stored.Water, computed.WaterUsed, computed.WaterCostRiel,
			value, computed.ElecUsed, computed.ElecCostRiel,
			computed.TotalRiel, computed.TotalUSD,
		); err != nil {
			return false, "", err
		}
		if err := s.pending.Clear(chatID); err != nil {
			return false, "", err
		}

		daysStayed := bill.DaysStayed
		if daysStayed <= 0 {
			daysStayed = daysInMonth
		}

		periodLabel := period.Label
		if err == nil {
			periodLabel = billing.KhmerMonthYear(int(periodMonth.Month()), periodMonth.Year())
		}

		invoice := billing.RenderKhmerInvoice(billing.InvoiceInput{
			PeriodLabel: periodLabel,
			RoomNumber:  bill.RoomNumber,
			DaysStayed:  daysStayed,
			DaysInMonth: daysInMonth,
			RentUSD:     bill.RentUSD,
			WaterPrev:   bill.WaterPrev,
			WaterCurr:   stored.Water,
			WaterUsed:   computed.WaterUsed,
			WaterCost:   computed.WaterCostRiel,
			ElecPrev:    bill.ElecPrev,
			ElecCurr:    value,
			ElecUsed:    computed.ElecUsed,
			ElecCost:    computed.ElecCostRiel,
			RentRiel:    computed.RentRiel,
			TotalRiel:   computed.TotalRiel,
			TotalUSD:    computed.TotalUSD,
		})

		return true, invoice, nil

	default:
		return false, "", fmt.Errorf("unknown pending step %q", pa.Step)
	}
}

func (s *Service) CancelPending(chatID int64) error {
	return s.pending.Clear(chatID)
}

func trimFloat(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}

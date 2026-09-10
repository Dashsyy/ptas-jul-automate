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
	"strconv"
	"strings"
	"time"

	"ptas-bot/internal/billing"
	"ptas-bot/internal/models"
	"ptas-bot/internal/store"
)

var ErrNoActivePeriod = errors.New("no active billing period — run NewMonth first")
var ErrNoPendingReading = errors.New("no reading in progress for this chat")

// paymentEpsilon absorbs float rounding (e.g. a $71.125 bill stored as
// 71.12999999) so a payment for the full display amount is never left
// dangling in "partial" by a fraction of a cent.
const paymentEpsilon = 0.005

type Service struct {
	rooms    *store.RoomStore
	periods  *store.PeriodStore
	bills    *store.BillStore
	pending  *store.PendingStore
	payments *store.PaymentStore
	rates    billing.Rates
	loc      *time.Location
}

func New(db *sql.DB, rates billing.Rates, loc *time.Location) *Service {
	return &Service{
		rooms:    store.NewRoomStore(db),
		periods:  store.NewPeriodStore(db),
		bills:    store.NewBillStore(db),
		pending:  store.NewPendingStore(db),
		payments: store.NewPaymentStore(db),
		rates:    rates,
		loc:      loc,
	}
}

func (s *Service) ListRooms() ([]models.Room, error) {
	return s.rooms.List()
}

func (s *Service) SetTenantName(roomNumber int, name string) error {
	return s.rooms.SetTenantName(roomNumber, name)
}

// resolvePeriodMonth returns the calendar month a period represents, parsed
// from its "YYYY-MM" label (falling back to when the period was created if
// the label isn't in that shape).
func (s *Service) resolvePeriodMonth(period models.BillingPeriod) time.Time {
	t, err := time.ParseInLocation("2006-01", period.Label, s.loc)
	if err != nil {
		return period.StartedAt.In(s.loc)
	}
	return t
}

// daysElapsedInPeriod returns how many days into the period's month "today"
// is (1..daysInMonth) — used by /vacate to auto-prorate a departing tenant's
// final bill without making the admin compute it by hand.
func (s *Service) daysElapsedInPeriod(period models.BillingPeriod) int {
	periodMonth := s.resolvePeriodMonth(period)
	daysInMonth := billing.DaysInMonth(periodMonth)
	now := time.Now().In(s.loc)
	if now.Year() != periodMonth.Year() || now.Month() != periodMonth.Month() {
		return daysInMonth
	}
	d := now.Day()
	if d > daysInMonth {
		d = daysInMonth
	}
	return d
}

// daysRemainingInPeriod returns how many days are left in the period's month
// from today through month-end (inclusive) — used by /movein to auto-prorate
// a new tenant's first, likely partial, month.
func (s *Service) daysRemainingInPeriod(period models.BillingPeriod) int {
	periodMonth := s.resolvePeriodMonth(period)
	daysInMonth := billing.DaysInMonth(periodMonth)
	now := time.Now().In(s.loc)
	if now.Year() != periodMonth.Year() || now.Month() != periodMonth.Month() {
		return daysInMonth
	}
	remaining := daysInMonth - now.Day() + 1
	if remaining < 1 {
		remaining = 1
	}
	return remaining
}

// StartVacateEntry begins moving a tenant out: it auto-prorates their final
// bill to today's date, then starts the same water/electricity reading
// conversation as /billing so that final bill is computed from a real
// reading rather than skipped. The room is only flagged vacant once both
// readings come in (see submitBillReading) — if no billing period is open
// yet, there's nothing to bill, so it's flagged vacant immediately instead.
func (s *Service) StartVacateEntry(chatID int64, roomNumber int) (string, error) {
	room, err := s.rooms.GetByNumber(roomNumber)
	if err != nil {
		return "", err
	}

	period, err := s.CurrentPeriod()
	if err == ErrNoActivePeriod {
		if err := s.rooms.SetVacant(roomNumber, true); err != nil {
			return "", err
		}
		if err := s.rooms.SetTenantName(roomNumber, ""); err != nil {
			return "", err
		}
		return fmt.Sprintf("🚪 Room %d marked vacant. No billing period is open, so there's nothing to bill.", room.Number), nil
	}
	if err != nil {
		return "", err
	}

	bill, err := s.bills.GetByRoomPeriod(room.ID, period.ID)
	if err != nil {
		return "", err
	}

	daysStayed := s.daysElapsedInPeriod(period)
	if err := s.bills.SetOccupancy(bill.ID, daysStayed); err != nil {
		return "", err
	}

	if err := s.pending.Set(models.PendingAction{
		ChatID: chatID, Kind: "vacate_readings", BillID: &bill.ID, Step: "await_water", Payload: "{}",
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("🚪 Room %d moving out — billed for %d day(s) this period.\n💧 Enter the WATER meter reading (previous: %s m³):",
		room.Number, daysStayed, trimFloat(bill.WaterPrev)), nil
}

// StartMoveIn begins moving a new tenant in: it unflags the room as vacant,
// auto-prorates the remainder of the period, then asks for the current
// water/electricity readings as the new tenant's starting baseline (stored
// as WaterPrev/ElecPrev — no bill is computed yet, that happens at the next
// normal /billing pass once their own current readings are known).
func (s *Service) StartMoveIn(chatID int64, roomNumber int) (string, error) {
	room, err := s.rooms.GetByNumber(roomNumber)
	if err != nil {
		return "", err
	}
	if err := s.rooms.SetVacant(roomNumber, false); err != nil {
		return "", err
	}

	period, err := s.CurrentPeriod()
	if err == ErrNoActivePeriod {
		return fmt.Sprintf("🔑 Room %d unmarked vacant. No billing period open yet — run /newmonth, then /movein %d again to record the starting meter readings.",
			room.Number, room.Number), nil
	}
	if err != nil {
		return "", err
	}

	bill, err := s.bills.GetByRoomPeriod(room.ID, period.ID)
	if err != nil {
		return "", err
	}

	daysStayed := s.daysRemainingInPeriod(period)
	if err := s.bills.SetOccupancy(bill.ID, daysStayed); err != nil {
		return "", err
	}

	if err := s.pending.Set(models.PendingAction{
		ChatID: chatID, Kind: "movein_baseline", BillID: &bill.ID, Step: "await_water", Payload: "{}",
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("🔑 Room %d moving in — billed for %d day(s) this period.\n💧 Enter the current WATER meter reading (new baseline):",
		room.Number, daysStayed), nil
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
		if b.Status == models.BillStatusUnpaid || b.Status == models.BillStatusPartial {
			unpaid = append(unpaid, b)
		}
	}
	return unpaid, nil
}

// AllBills lists every room's bill in the current period, in room-number
// order, regardless of status — for a full paid/unpaid overview.
func (s *Service) AllBills() ([]models.Bill, error) {
	period, err := s.CurrentPeriod()
	if err != nil {
		return nil, err
	}
	return s.bills.ListForPeriod(period.ID)
}

// RoomsMissingReadings lists bills in the current period that haven't had
// meter readings entered yet — no_charge bills (vacant rooms included) are
// excluded, since nobody's there to generate usage worth recording.
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
		if b.WaterCurr == nil && b.Status != models.BillStatusNoCharge {
			missing = append(missing, b)
		}
	}
	return missing, nil
}

// RecordPayment logs a payment toward a bill and recomputes its status
// (unpaid -> partial -> paid) from the running total of payments logged
// against it. Returns the updated bill.
func (s *Service) RecordPayment(billID int64, amountUSD float64, note string) (models.Bill, error) {
	if amountUSD <= 0 {
		return models.Bill{}, fmt.Errorf("payment amount must be positive")
	}

	bill, err := s.bills.GetByID(billID)
	if err != nil {
		return models.Bill{}, err
	}
	if bill.Status == models.BillStatusNoCharge {
		return models.Bill{}, fmt.Errorf("room %d is marked no-charge — no payment expected", bill.RoomNumber)
	}

	if err := s.payments.Add(billID, amountUSD, note); err != nil {
		return models.Bill{}, err
	}

	paidTotal, err := s.payments.TotalForBill(billID)
	if err != nil {
		return models.Bill{}, err
	}

	var status models.BillStatus
	var paidAt *time.Time
	switch {
	case paidTotal+paymentEpsilon >= bill.TotalUSD:
		status = models.BillStatusPaid
		now := time.Now()
		paidAt = &now
	case paidTotal > 0:
		status = models.BillStatusPartial
	default:
		status = models.BillStatusUnpaid
	}

	if err := s.bills.SetStatus(billID, status, paidAt); err != nil {
		return models.Bill{}, err
	}
	return s.bills.GetByID(billID)
}

// MarkPaid settles whatever remains on a bill in a single payment — the
// /unpaid tap-to-pay shortcut for the common "paid in full" case.
func (s *Service) MarkPaid(billID int64) (models.Bill, error) {
	bill, err := s.bills.GetByID(billID)
	if err != nil {
		return models.Bill{}, err
	}
	remaining := bill.TotalUSD - bill.PaidUSD
	if remaining <= paymentEpsilon {
		return bill, nil
	}
	return s.RecordPayment(billID, remaining, "settled in full via /unpaid")
}

// RoomStatus fetches a room's bill for the current period by room number —
// used by /pay and the @mention status query.
func (s *Service) RoomStatus(roomNumber int) (models.Bill, error) {
	room, err := s.rooms.GetByNumber(roomNumber)
	if err != nil {
		return models.Bill{}, err
	}
	period, err := s.CurrentPeriod()
	if err != nil {
		return models.Bill{}, err
	}
	return s.bills.GetByRoomPeriod(room.ID, period.ID)
}

// PayRoom records a payment against a room's current-period bill by room
// number — the entry point for /pay <room#> <amount>.
func (s *Service) PayRoom(roomNumber int, amountUSD float64) (models.Bill, error) {
	bill, err := s.RoomStatus(roomNumber)
	if err != nil {
		return models.Bill{}, err
	}
	return s.RecordPayment(bill.ID, amountUSD, "")
}

// StartPayFlow begins the "how much did Room N pay?" conversation for a
// room already chosen (by tapping a room button) — the interactive
// counterpart to /pay <room#> <amount>.
func (s *Service) StartPayFlow(chatID int64, roomNumber int) (string, error) {
	bill, err := s.RoomStatus(roomNumber)
	if err != nil {
		return "", err
	}
	if err := s.pending.Set(models.PendingAction{
		ChatID: chatID, Kind: "pay_amount", BillID: &bill.ID, Step: "await_amount", Payload: "{}",
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("💵 Room %d — how much did they pay (owes $%.2f)?", roomNumber, bill.TotalUSD-bill.PaidUSD), nil
}

func (s *Service) submitPayAmount(pa models.PendingAction, amount float64) (bool, string, error) {
	if err := s.pending.Clear(pa.ChatID); err != nil {
		return false, "", err
	}
	bill, err := s.RecordPayment(*pa.BillID, amount, "")
	if err != nil {
		return false, "", err
	}
	return true, fmt.Sprintf("💵 Logged $%.2f for Room %d.", amount, bill.RoomNumber), nil
}

// StartSetNameFlow begins the "what name?" conversation for a room already
// chosen — the interactive counterpart to /setname <room#> <name>. It only
// needs a room, not a bill, so /setname works even before any billing
// period has ever been opened.
func (s *Service) StartSetNameFlow(chatID int64, roomNumber int) (string, error) {
	room, err := s.rooms.GetByNumber(roomNumber)
	if err != nil {
		return "", err
	}
	if err := s.pending.Set(models.PendingAction{
		ChatID: chatID, Kind: "setname_text", RoomID: &room.ID, Step: "await_name", Payload: "{}",
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("✏️ Room %d — what's the tenant's name?", roomNumber), nil
}

func (s *Service) submitSetName(pa models.PendingAction, name string) (bool, string, error) {
	if name == "" {
		return false, "Name can't be empty — try again.", nil
	}
	room, err := s.rooms.GetByID(*pa.RoomID)
	if err != nil {
		return false, "", err
	}
	if err := s.pending.Clear(pa.ChatID); err != nil {
		return false, "", err
	}
	if err := s.rooms.SetTenantName(room.Number, name); err != nil {
		return false, "", err
	}
	return true, fmt.Sprintf("Room %d name set to %q.", room.Number, name), nil
}

// StartNewMonthFlow begins the "what label?" conversation for opening a new
// billing period — the interactive counterpart to /newmonth 2026-10.
func (s *Service) StartNewMonthFlow(chatID int64) (string, error) {
	if err := s.pending.Set(models.PendingAction{
		ChatID: chatID, Kind: "newmonth_label", Step: "await_label", Payload: "{}",
	}); err != nil {
		return "", err
	}
	return "📅 What label for the new period? e.g. 2026-10", nil
}

func (s *Service) submitNewMonthLabel(pa models.PendingAction, label string) (bool, string, error) {
	if label == "" {
		return false, "Label can't be empty — try again, e.g. 2026-10.", nil
	}
	if err := s.pending.Clear(pa.ChatID); err != nil {
		return false, "", err
	}
	count, err := s.NewMonth(label)
	if err != nil {
		return false, "", err
	}
	return true, fmt.Sprintf("📅 Opened period %s with %d rooms.", label, count), nil
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
		BillID:  &bill.ID,
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

// SubmitTextReply advances whatever conversation is pending for a chat —
// meter readings, a /vacate final reading, a /movein baseline, a /pay
// amount, a /setname reply, or a /newmonth label all flow through here,
// dispatched by the pending action's Kind. Returns done=true once the
// conversation is complete, with message holding whatever's appropriate to
// show (an invoice, a confirmation, or a "that's not a number" nudge).
func (s *Service) SubmitTextReply(chatID int64, text string) (done bool, message string, err error) {
	pa, err := s.pending.Get(chatID)
	if err == sql.ErrNoRows {
		return false, "", ErrNoPendingReading
	}
	if err != nil {
		return false, "", err
	}

	switch pa.Kind {
	case "enter_readings", "vacate_readings":
		value, perr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if perr != nil {
			return false, "Please enter a number.", nil
		}
		return s.submitBillReading(pa, value)
	case "movein_baseline":
		value, perr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if perr != nil {
			return false, "Please enter a number.", nil
		}
		return s.submitMoveInReading(pa, value)
	case "pay_amount":
		value, perr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if perr != nil {
			return false, "Please enter a number.", nil
		}
		return s.submitPayAmount(pa, value)
	case "setname_text":
		return s.submitSetName(pa, strings.TrimSpace(text))
	case "newmonth_label":
		return s.submitNewMonthLabel(pa, strings.TrimSpace(text))
	default:
		return false, "", fmt.Errorf("unknown pending kind %q", pa.Kind)
	}
}

// submitBillReading handles the two-step water/electricity conversation for
// both a normal period bill (Kind "enter_readings") and a departing tenant's
// final prorated bill (Kind "vacate_readings"). In the latter case, the room
// is flagged vacant only once the bill is actually computed and saved.
func (s *Service) submitBillReading(pa models.PendingAction, value float64) (bool, string, error) {
	bill, err := s.bills.GetByID(*pa.BillID)
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
		// Use the period's own month, not the current wall-clock date — a
		// bill for August must prorate against August's day count even if
		// it's actually being entered in September.
		periodMonth := s.resolvePeriodMonth(period)
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
		if err := s.pending.Clear(pa.ChatID); err != nil {
			return false, "", err
		}

		daysStayed := bill.DaysStayed
		if daysStayed <= 0 {
			daysStayed = daysInMonth
		}

		invoice := billing.RenderKhmerInvoice(billing.InvoiceInput{
			PeriodLabel: billing.KhmerMonthYear(int(periodMonth.Month()), periodMonth.Year()),
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

		if pa.Kind == "vacate_readings" {
			room, err := s.rooms.GetByID(bill.RoomID)
			if err == nil {
				_ = s.rooms.SetVacant(room.Number, true)
				_ = s.rooms.SetTenantName(room.Number, "")
			}
			invoice += "\n\n🚪 Room now marked vacant."
		}

		return true, "✅ Bill saved.\n\n" + invoice, nil

	default:
		return false, "", fmt.Errorf("unknown pending step %q", pa.Step)
	}
}

// submitMoveInReading handles /movein's two-step conversation, which writes
// the new tenant's starting water/electricity readings as the bill's
// previous readings rather than computing a bill — that happens later, at
// the normal end-of-period /billing pass.
func (s *Service) submitMoveInReading(pa models.PendingAction, value float64) (bool, string, error) {
	bill, err := s.bills.GetByID(*pa.BillID)
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
		return false, fmt.Sprintf("⚡ Room %d — enter the current ELECTRICITY meter reading (new baseline):", bill.RoomNumber), nil

	case "await_elec":
		var stored readingPayload
		if err := json.Unmarshal([]byte(pa.Payload), &stored); err != nil {
			return false, "", err
		}
		if err := s.bills.SetPreviousReadings(bill.ID, stored.Water, value); err != nil {
			return false, "", err
		}
		if err := s.pending.Clear(pa.ChatID); err != nil {
			return false, "", err
		}
		return true, fmt.Sprintf("✅ Room %d ready — starting readings recorded (water %s m³, electricity %s kWh). It'll bill normally at the next /billing pass.",
			bill.RoomNumber, trimFloat(stored.Water), trimFloat(value)), nil

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

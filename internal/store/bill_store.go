package store

import (
	"database/sql"
	"time"

	"ptas-bot/internal/models"
)

type BillStore struct {
	db *sql.DB
}

func NewBillStore(db *sql.DB) *BillStore {
	return &BillStore{db: db}
}

const billSelectCols = `
	b.id, b.room_id, b.period_id, b.days_stayed, b.rent_usd,
	b.water_prev, b.water_curr, b.water_used, b.water_cost_riel,
	b.elec_prev, b.elec_curr, b.elec_used, b.elec_cost_riel,
	b.extra_charge_usd, b.total_riel, b.total_usd, b.status, b.notes, b.paid_at,
	r.number, r.floor, r.tenant_name,
	COALESCE((SELECT SUM(amount_usd) FROM payments WHERE bill_id = b.id), 0)`

func scanBill(row interface{ Scan(dest ...any) error }) (models.Bill, error) {
	var b models.Bill
	err := row.Scan(
		&b.ID, &b.RoomID, &b.PeriodID, &b.DaysStayed, &b.RentUSD,
		&b.WaterPrev, &b.WaterCurr, &b.WaterUsed, &b.WaterCostRiel,
		&b.ElecPrev, &b.ElecCurr, &b.ElecUsed, &b.ElecCostRiel,
		&b.ExtraChargeUSD, &b.TotalRiel, &b.TotalUSD, &b.Status, &b.Notes, &b.PaidAt,
		&b.RoomNumber, &b.RoomFloor, &b.TenantName, &b.PaidUSD,
	)
	return b, err
}

// EnsureForPeriod creates a blank bill row for a room in a period if one
// doesn't already exist, carrying the previous period's "current" readings
// forward as this period's "previous" readings. A room still marked vacant
// (via /vacate, not yet reoccupied with /movein) gets a no_charge bill
// automatically instead of an unpaid one.
func (s *BillStore) EnsureForPeriod(room models.Room, periodID int64, prevWater, prevElec float64) error {
	status := models.BillStatusUnpaid
	notes := ""
	if room.IsVacant {
		status = models.BillStatusNoCharge
		notes = "vacant"
	}
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO bills (room_id, period_id, rent_usd, water_prev, elec_prev, status, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		room.ID, periodID, room.BaseRentUSD, prevWater, prevElec, status, notes)
	return err
}

func (s *BillStore) LastReadingsForRoom(roomID int64) (water, elec float64, err error) {
	err = s.db.QueryRow(`
		SELECT COALESCE(water_curr, water_prev), COALESCE(elec_curr, elec_prev)
		FROM bills WHERE room_id = ? ORDER BY period_id DESC LIMIT 1`, roomID).
		Scan(&water, &elec)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return water, elec, err
}

func (s *BillStore) ListForPeriod(periodID int64) ([]models.Bill, error) {
	rows, err := s.db.Query(`
		SELECT `+billSelectCols+`
		FROM bills b JOIN rooms r ON r.id = b.room_id
		WHERE b.period_id = ? ORDER BY r.number`, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bills []models.Bill
	for rows.Next() {
		b, err := scanBill(rows)
		if err != nil {
			return nil, err
		}
		bills = append(bills, b)
	}
	return bills, rows.Err()
}

func (s *BillStore) GetByRoomPeriod(roomID, periodID int64) (models.Bill, error) {
	row := s.db.QueryRow(`
		SELECT `+billSelectCols+`
		FROM bills b JOIN rooms r ON r.id = b.room_id
		WHERE b.room_id = ? AND b.period_id = ?`, roomID, periodID)
	return scanBill(row)
}

func (s *BillStore) GetByID(id int64) (models.Bill, error) {
	row := s.db.QueryRow(`
		SELECT `+billSelectCols+`
		FROM bills b JOIN rooms r ON r.id = b.room_id
		WHERE b.id = ?`, id)
	return scanBill(row)
}

// SetStatus updates a bill's status (and paid_at, when settling in full) —
// used by the service layer after recomputing status from logged payments.
func (s *BillStore) SetStatus(id int64, status models.BillStatus, paidAt *time.Time) error {
	_, err := s.db.Exec(`UPDATE bills SET status = ?, paid_at = ? WHERE id = ?`, status, paidAt, id)
	return err
}

// SetOccupancy resets a bill back to unpaid with the given days-stayed —
// used by /vacate and /movein to auto-prorate the day a tenant leaves or
// arrives.
func (s *BillStore) SetOccupancy(id int64, daysStayed int) error {
	_, err := s.db.Exec(`UPDATE bills SET days_stayed = ?, status = 'unpaid', notes = '' WHERE id = ?`, daysStayed, id)
	return err
}

// SetPreviousReadings overwrites a bill's starting (previous) meter readings
// without touching its status or totals — used by /movein to record a new
// tenant's baseline; the bill itself still gets computed later, once their
// current readings are known at the normal /billing pass.
func (s *BillStore) SetPreviousReadings(id int64, waterPrev, elecPrev float64) error {
	_, err := s.db.Exec(`UPDATE bills SET water_prev = ?, elec_prev = ? WHERE id = ?`, waterPrev, elecPrev, id)
	return err
}

// SaveReadings stores the computed usage/cost/total for a bill after both
// meter readings have been entered.
func (s *BillStore) SaveReadings(id int64, waterCurr, waterUsed, waterCost, elecCurr, elecUsed, elecCost, totalRiel, totalUSD float64) error {
	_, err := s.db.Exec(`
		UPDATE bills SET
			water_curr = ?, water_used = ?, water_cost_riel = ?,
			elec_curr = ?, elec_used = ?, elec_cost_riel = ?,
			total_riel = ?, total_usd = ?
		WHERE id = ?`,
		waterCurr, waterUsed, waterCost, elecCurr, elecUsed, elecCost, totalRiel, totalUSD, id)
	return err
}

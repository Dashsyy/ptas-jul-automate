package models

import "time"

type Room struct {
	ID          int64
	Number      int
	Floor       int
	TenantName  string
	BaseRentUSD float64
}

type BillingPeriod struct {
	ID        int64
	Label     string // e.g. "2026-08"
	StartedAt time.Time
}

type BillStatus string

const (
	BillStatusUnpaid   BillStatus = "unpaid"
	BillStatusPaid     BillStatus = "paid"
	BillStatusNoCharge BillStatus = "no_charge"
)

type Bill struct {
	ID             int64
	RoomID         int64
	PeriodID       int64
	DaysStayed     int
	RentUSD        float64
	WaterPrev      float64
	WaterCurr      *float64
	WaterUsed      float64
	WaterCostRiel  float64
	ElecPrev       float64
	ElecCurr       *float64
	ElecUsed       float64
	ElecCostRiel   float64
	ExtraChargeUSD float64
	TotalRiel      float64
	TotalUSD       float64
	Status         BillStatus
	Notes          string
	PaidAt         *time.Time

	// populated by joins, not persisted directly on this table
	RoomNumber int
	TenantName string
}

// PendingAction tracks a multi-step conversation (e.g. entering meter readings)
// per chat, so it survives process restarts.
type PendingAction struct {
	ChatID    int64
	Kind      string // "enter_readings"
	BillID    int64
	Step      string // "await_water", "await_elec"
	Payload   string // JSON blob for partially collected data
	UpdatedAt time.Time
}

type PaymentLog struct {
	ID            int64
	RawText       string
	AmountUSD     float64
	PayerName     string
	TrxID         string
	APV           string
	OccurredAt    time.Time
	MatchedBillID *int64
}

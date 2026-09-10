package models

import "time"

type Room struct {
	ID          int64
	Number      int
	Floor       int
	TenantName  string
	BaseRentUSD float64
	IsVacant    bool
}

type BillingPeriod struct {
	ID        int64
	Label     string // e.g. "2026-08"
	StartedAt time.Time
}

type BillStatus string

const (
	BillStatusUnpaid   BillStatus = "unpaid"
	BillStatusPartial  BillStatus = "partial"
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

	// populated by joins/aggregates, not persisted directly on this table
	RoomNumber int
	RoomFloor  int
	TenantName string
	PaidUSD    float64 // sum of payments logged against this bill
}

// Payment is one payment logged toward a bill — a bill can have several,
// which is what makes split/partial payments possible.
type Payment struct {
	ID        int64
	BillID    int64
	AmountUSD float64
	Note      string
	PaidAt    time.Time
}

// PendingAction tracks a multi-step conversation (e.g. entering meter
// readings, or a bare "/pay" walking through room-then-amount) per chat, so
// it survives process restarts. BillID and RoomID are both optional — which
// one (if either) is set depends on Kind: reading/payment flows anchor to a
// bill, /setname and /newmonth flows may only need a room or nothing at all.
type PendingAction struct {
	ChatID    int64
	Kind      string // "enter_readings", "pay_amount", "setname_text", ...
	BillID    *int64
	RoomID    *int64
	Step      string
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

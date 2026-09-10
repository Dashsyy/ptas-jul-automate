package store

import (
	"database/sql"
	"time"
)

type PaymentStore struct {
	db *sql.DB
}

func NewPaymentStore(db *sql.DB) *PaymentStore {
	return &PaymentStore{db: db}
}

func (s *PaymentStore) Add(billID int64, amountUSD float64, note string) error {
	_, err := s.db.Exec(`INSERT INTO payments (bill_id, amount_usd, note, paid_at) VALUES (?, ?, ?, ?)`,
		billID, amountUSD, note, time.Now())
	return err
}

func (s *PaymentStore) TotalForBill(billID int64) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount_usd) FROM payments WHERE bill_id = ?`, billID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

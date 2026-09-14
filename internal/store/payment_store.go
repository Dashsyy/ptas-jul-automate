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

// Add records a payment. externalRef is a source-system identifier (e.g. an
// ABA PayWay transaction ID) used to detect duplicates — pass "" for a
// manually-entered payment that has no such identifier.
func (s *PaymentStore) Add(billID int64, amountUSD float64, note, externalRef string) error {
	_, err := s.db.Exec(`INSERT INTO payments (bill_id, amount_usd, note, external_ref, paid_at) VALUES (?, ?, ?, ?, ?)`,
		billID, amountUSD, note, externalRef, time.Now())
	return err
}

// ExistsByExternalRef reports whether a payment with this external
// reference has already been recorded — always false for an empty ref.
func (s *PaymentStore) ExistsByExternalRef(externalRef string) (bool, error) {
	if externalRef == "" {
		return false, nil
	}
	var found int
	err := s.db.QueryRow(`SELECT 1 FROM payments WHERE external_ref = ? LIMIT 1`, externalRef).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *PaymentStore) TotalForBill(billID int64) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount_usd) FROM payments WHERE bill_id = ?`, billID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

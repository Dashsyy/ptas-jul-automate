package store

import (
	"database/sql"
	"time"

	"ptas-bot/internal/models"
)

type PeriodStore struct {
	db *sql.DB
}

func NewPeriodStore(db *sql.DB) *PeriodStore {
	return &PeriodStore{db: db}
}

// Current returns the most recently started billing period, or sql.ErrNoRows
// if /newmonth has never been run.
func (s *PeriodStore) Current() (models.BillingPeriod, error) {
	var p models.BillingPeriod
	err := s.db.QueryRow(`SELECT id, label, started_at FROM billing_periods ORDER BY started_at DESC LIMIT 1`).
		Scan(&p.ID, &p.Label, &p.StartedAt)
	return p, err
}

func (s *PeriodStore) GetByID(id int64) (models.BillingPeriod, error) {
	var p models.BillingPeriod
	err := s.db.QueryRow(`SELECT id, label, started_at FROM billing_periods WHERE id = ?`, id).
		Scan(&p.ID, &p.Label, &p.StartedAt)
	return p, err
}

func (s *PeriodStore) GetByLabel(label string) (models.BillingPeriod, error) {
	var p models.BillingPeriod
	err := s.db.QueryRow(`SELECT id, label, started_at FROM billing_periods WHERE label = ?`, label).
		Scan(&p.ID, &p.Label, &p.StartedAt)
	return p, err
}

func (s *PeriodStore) Create(label string, startedAt time.Time) (models.BillingPeriod, error) {
	res, err := s.db.Exec(`INSERT INTO billing_periods (label, started_at) VALUES (?, ?)`, label, startedAt)
	if err != nil {
		return models.BillingPeriod{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.BillingPeriod{}, err
	}
	return models.BillingPeriod{ID: id, Label: label, StartedAt: startedAt}, nil
}

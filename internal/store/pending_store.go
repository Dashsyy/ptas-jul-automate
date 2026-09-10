package store

import (
	"database/sql"
	"time"

	"ptas-bot/internal/models"
)

type PendingStore struct {
	db *sql.DB
}

func NewPendingStore(db *sql.DB) *PendingStore {
	return &PendingStore{db: db}
}

func (s *PendingStore) Get(chatID int64) (models.PendingAction, error) {
	var p models.PendingAction
	var billID, roomID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT chat_id, kind, bill_id, room_id, step, payload, updated_at
		FROM pending_actions WHERE chat_id = ?`, chatID).
		Scan(&p.ChatID, &p.Kind, &billID, &roomID, &p.Step, &p.Payload, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	if billID.Valid {
		p.BillID = &billID.Int64
	}
	if roomID.Valid {
		p.RoomID = &roomID.Int64
	}
	return p, nil
}

func (s *PendingStore) Set(p models.PendingAction) error {
	var billID, roomID any
	if p.BillID != nil {
		billID = *p.BillID
	}
	if p.RoomID != nil {
		roomID = *p.RoomID
	}
	_, err := s.db.Exec(`
		INSERT INTO pending_actions (chat_id, kind, bill_id, room_id, step, payload, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			kind = excluded.kind, bill_id = excluded.bill_id, room_id = excluded.room_id,
			step = excluded.step, payload = excluded.payload, updated_at = excluded.updated_at`,
		p.ChatID, p.Kind, billID, roomID, p.Step, p.Payload, time.Now())
	return err
}

func (s *PendingStore) Clear(chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM pending_actions WHERE chat_id = ?`, chatID)
	return err
}

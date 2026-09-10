package store

import (
	"database/sql"
	"fmt"

	"ptas-bot/internal/models"
)

type RoomStore struct {
	db *sql.DB
}

func NewRoomStore(db *sql.DB) *RoomStore {
	return &RoomStore{db: db}
}

func (s *RoomStore) List() ([]models.Room, error) {
	rows, err := s.db.Query(`SELECT id, number, floor, tenant_name, base_rent_usd FROM rooms ORDER BY number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []models.Room
	for rows.Next() {
		var r models.Room
		if err := rows.Scan(&r.ID, &r.Number, &r.Floor, &r.TenantName, &r.BaseRentUSD); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (s *RoomStore) GetByNumber(number int) (models.Room, error) {
	var r models.Room
	err := s.db.QueryRow(`SELECT id, number, floor, tenant_name, base_rent_usd FROM rooms WHERE number = ?`, number).
		Scan(&r.ID, &r.Number, &r.Floor, &r.TenantName, &r.BaseRentUSD)
	if err != nil {
		return r, fmt.Errorf("room %d: %w", number, err)
	}
	return r, nil
}

func (s *RoomStore) GetByID(id int64) (models.Room, error) {
	var r models.Room
	err := s.db.QueryRow(`SELECT id, number, floor, tenant_name, base_rent_usd FROM rooms WHERE id = ?`, id).
		Scan(&r.ID, &r.Number, &r.Floor, &r.TenantName, &r.BaseRentUSD)
	return r, err
}

func (s *RoomStore) SetTenantName(number int, name string) error {
	res, err := s.db.Exec(`UPDATE rooms SET tenant_name = ? WHERE number = ?`, name, number)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("room %d not found", number)
	}
	return nil
}

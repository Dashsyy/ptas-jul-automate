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

const roomSelectCols = `id, number, floor, tenant_name, base_rent_usd, is_vacant`

func scanRoom(row interface{ Scan(dest ...any) error }) (models.Room, error) {
	var r models.Room
	err := row.Scan(&r.ID, &r.Number, &r.Floor, &r.TenantName, &r.BaseRentUSD, &r.IsVacant)
	return r, err
}

func (s *RoomStore) List() ([]models.Room, error) {
	rows, err := s.db.Query(`SELECT ` + roomSelectCols + ` FROM rooms ORDER BY number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []models.Room
	for rows.Next() {
		r, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (s *RoomStore) GetByNumber(number int) (models.Room, error) {
	row := s.db.QueryRow(`SELECT `+roomSelectCols+` FROM rooms WHERE number = ?`, number)
	r, err := scanRoom(row)
	if err != nil {
		return r, fmt.Errorf("room %d: %w", number, err)
	}
	return r, nil
}

func (s *RoomStore) GetByID(id int64) (models.Room, error) {
	row := s.db.QueryRow(`SELECT `+roomSelectCols+` FROM rooms WHERE id = ?`, id)
	return scanRoom(row)
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

func (s *RoomStore) SetVacant(number int, vacant bool) error {
	res, err := s.db.Exec(`UPDATE rooms SET is_vacant = ? WHERE number = ?`, vacant, number)
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

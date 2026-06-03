package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linkall/server/internal/db"
)

type DeviceSession struct {
	ID           string `json:"id"`
	ControllerID string `json:"controller_id"`
	ControlledID int64  `json:"controlled_id"`
	StartedAt    int64  `json:"started_at"`
	LastActive   int64  `json:"last_active"`
	Closed       bool   `json:"closed"`
	BytesTx      int64  `json:"bytes_tx"`
	BytesRx      int64  `json:"bytes_rx"`
	RelayUsed    bool   `json:"relay_used"`
}

func StartSession(controllerID string, controlledID int64, relay bool) (*DeviceSession, error) {
	s := &DeviceSession{
		ID:           uuid.NewString(),
		ControllerID: controllerID,
		ControlledID: controlledID,
		StartedAt:    time.Now().Unix(),
		LastActive:   time.Now().Unix(),
		RelayUsed:    relay,
	}
	_, err := db.DB.Exec(
		`INSERT INTO device_sessions(id, controller_id, controlled_id, started_at, last_active, closed, relay_used) VALUES(?,?,?,?,?,0,?)`,
		s.ID, s.ControllerID, s.ControlledID, s.StartedAt, s.LastActive, boolToInt(relay),
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func TouchSession(id string, deltaTx, deltaRx int64) error {
	_, err := db.DB.Exec(
		`UPDATE device_sessions SET last_active=?, bytes_tx=bytes_tx+?, bytes_rx=bytes_rx+? WHERE id=? AND closed=0`,
		time.Now().Unix(), deltaTx, deltaRx, id,
	)
	return err
}

func CloseSession(id string) error {
	res, err := db.DB.Exec(`UPDATE device_sessions SET closed=1, last_active=? WHERE id=? AND closed=0`,
		time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("session not open")
	}
	return nil
}

func GetSession(id string) (*DeviceSession, error) {
	row := db.DB.QueryRow(
		`SELECT id, controller_id, controlled_id, started_at, last_active, closed, bytes_tx, bytes_rx, relay_used FROM device_sessions WHERE id=?`, id,
	)
	var s DeviceSession
	var closed, relay int
	if err := row.Scan(&s.ID, &s.ControllerID, &s.ControlledID, &s.StartedAt, &s.LastActive, &closed, &s.BytesTx, &s.BytesRx, &relay); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("session not found")
		}
		return nil, err
	}
	s.Closed = closed != 0
	s.RelayUsed = relay != 0
	return &s, nil
}


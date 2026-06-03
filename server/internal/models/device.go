package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/db"
)

type Device struct {
	ID                 int64   `json:"id"`
	OwnerID            *int64  `json:"owner_id,omitempty"`
	DeviceCode         string  `json:"device_code"`
	Name               string  `json:"name"`
	Platform           string  `json:"platform"`
	OSVersion          string  `json:"os_version"`
	AppVersion         string  `json:"app_version"`
	AllowAnonymous     bool    `json:"allow_anonymous"`
	RequireDeviceCode  bool    `json:"require_device_code"`
	AcceptConnections  bool    `json:"accept_connections"`
	LastIP             string  `json:"last_ip"`
	LastSeen           int64   `json:"last_seen"`
	CreatedAt          int64   `json:"created_at"`
	Online             bool    `json:"online"`
	Tag                string  `json:"tag"`
	Notes              string  `json:"notes"`
}

func (d *Device) Scan(rs interface{ Scan(...any) error }) error {
	var ownerID sql.NullInt64
	var allowAn, reqCode, accept, online int
	var lastSeen sql.NullInt64
	var lastIP, name, osv, appv, platform, tag, notes sql.NullString
	if err := rs.Scan(&d.ID, &ownerID, &d.DeviceCode, &name, &platform, &osv, &appv, &allowAn, &reqCode, &accept, &lastIP, &lastSeen, &d.CreatedAt, &online, &tag, &notes); err != nil {
		return err
	}
	if ownerID.Valid {
		v := ownerID.Int64
		d.OwnerID = &v
	}
	d.Name = name.String
	d.Platform = platform.String
	d.OSVersion = osv.String
	d.AppVersion = appv.String
	d.AllowAnonymous = allowAn != 0
	d.RequireDeviceCode = reqCode != 0
	d.AcceptConnections = accept != 0
	d.LastIP = lastIP.String
	if lastSeen.Valid {
		d.LastSeen = lastSeen.Int64
	}
	d.Online = online != 0
	d.Tag = tag.String
	d.Notes = notes.String
	return nil
}

const deviceCols = `id, owner_id, device_code, COALESCE(name,''), COALESCE(platform,''), COALESCE(os_version,''), COALESCE(app_version,''),
allow_anonymous, require_device_code, accept_connections, COALESCE(last_ip,''), COALESCE(last_seen,0), created_at, online, COALESCE(tag,''), COALESCE(notes,'')`

func RegisterDevice(d *Device, deviceCode, devicePassword string) error {
	if deviceCode == "" {
		return errors.New("device_code 必填")
	}
	ph, err := auth.HashPassword(devicePassword, auth.DefaultParams())
	if err != nil {
		return err
	}
	d.DeviceCode = strings.ToUpper(strings.TrimSpace(d.DeviceCode))
	now := time.Now().Unix()
	if d.CreatedAt == 0 {
		d.CreatedAt = now
	}
	res, err := db.DB.Exec(
		`INSERT INTO devices(device_code, password_hash, name, platform, os_version, app_version, allow_anonymous, require_device_code, accept_connections, owner_id, created_at, last_seen, last_ip, online, tag, notes)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.DeviceCode, ph, d.Name, d.Platform, d.OSVersion, d.AppVersion,
		boolToInt(d.AllowAnonymous), boolToInt(d.RequireDeviceCode), boolToInt(d.AcceptConnections),
		d.OwnerID, d.CreatedAt, now, d.LastIP, 0, d.Tag, d.Notes,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	d.ID = id
	return nil
}

func FindDeviceByCode(code string) (*Device, string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	row := db.DB.QueryRow(
		`SELECT `+deviceCols+` FROM devices WHERE device_code=?`, code,
	)
	d := &Device{}
	if err := d.Scan(row); err != nil {
		return nil, "", err
	}
	// also fetch password hash
	var ph string
	_ = db.DB.QueryRow(`SELECT password_hash FROM devices WHERE device_code=?`, code).Scan(&ph)
	return d, ph, nil
}

func FindDeviceByID(id int64) (*Device, error) {
	row := db.DB.QueryRow(`SELECT `+deviceCols+` FROM devices WHERE id=?`, id)
	d := &Device{}
	if err := d.Scan(row); err != nil {
		return nil, err
	}
	return d, nil
}

func ListDevicesByOwner(ownerID int64) ([]Device, error) {
	rows, err := db.DB.Query(
		`SELECT `+deviceCols+` FROM devices WHERE owner_id=? ORDER BY id DESC`, ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		d := Device{}
		if err := d.Scan(rows); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func ListAllDevices() ([]Device, error) {
	rows, err := db.DB.Query(`SELECT ` + deviceCols + ` FROM devices ORDER BY id DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		d := Device{}
		if err := d.Scan(rows); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func SetDeviceOnline(deviceCode string, online bool) {
	_, _ = db.DB.Exec(
		`UPDATE devices SET online=?, last_seen=? WHERE device_code=?`,
		boolToInt(online), time.Now().Unix(), strings.ToUpper(deviceCode),
	)
}

func UpdateDeviceMeta(deviceCode, name, os, app, ip, tag, notes string) error {
	_, err := db.DB.Exec(
		`UPDATE devices SET name=COALESCE(NULLIF(?,''), name), os_version=COALESCE(NULLIF(?,''), os_version),
		 app_version=COALESCE(NULLIF(?,''), app_version), last_ip=COALESCE(NULLIF(?,''), last_ip),
		 last_seen=?, tag=COALESCE(NULLIF(?,''), tag), notes=COALESCE(NULLIF(?,''), notes), online=1 WHERE device_code=?`,
		name, os, app, ip, time.Now().Unix(), tag, notes, strings.ToUpper(deviceCode),
	)
	return err
}

func UpdateDeviceFlags(id int64, anon, reqCode, accept *bool) error {
	q := `UPDATE devices SET `
	args := []any{}
	first := true
	if anon != nil {
		if !first {
			q += ", "
		}
		q += `allow_anonymous=?`
		args = append(args, boolToInt(*anon))
		first = false
	}
	if reqCode != nil {
		if !first {
			q += ", "
		}
		q += `require_device_code=?`
		args = append(args, boolToInt(*reqCode))
		first = false
	}
	if accept != nil {
		if !first {
			q += ", "
		}
		q += `accept_connections=?`
		args = append(args, boolToInt(*accept))
		first = false
	}
	q += ` WHERE id=?`
	args = append(args, id)
	_, err := db.DB.Exec(q, args...)
	return err
}

func ResetDeviceCode(id int64, newCode, newPassword string) error {
	ph, err := auth.HashPassword(newPassword, auth.DefaultParams())
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE devices SET device_code=?, password_hash=? WHERE id=?`,
		strings.ToUpper(newCode), ph, id)
	return err
}

func CountDevices() (int, error) {
	var n int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&n)
	return n, err
}

func CountOnlineDevices() (int, error) {
	var n int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM devices WHERE online=1`).Scan(&n)
	return n, err
}


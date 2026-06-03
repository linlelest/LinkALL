package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/linkall/server/internal/db"
)

type OTAPackage struct {
	ID                  int64  `json:"id"`
	Platform            string `json:"platform"`
	Version             string `json:"version"`
	Channel             string `json:"channel"`
	FileName            string `json:"file_name"`
	FilePath            string `json:"file_path,omitempty"`
	FileSize            int64  `json:"file_size"`
	SHA256              string `json:"sha256"`
	Signature           string `json:"signature"`
	ReleaseNotes        string `json:"release_notes"`
	ForceUpdate         bool   `json:"force_update"`
	MinSupportedVersion string `json:"min_supported_version"`
	Downloads           int64  `json:"downloads"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
	Revoked             bool   `json:"revoked"`
}

func CreateOTA(p *OTAPackage) error {
	if p.Platform == "" || p.Version == "" {
		return errors.New("platform 和 version 必填")
	}
	p.Platform = strings.ToLower(strings.TrimSpace(p.Platform))
	p.Version = strings.TrimSpace(p.Version)
	p.Channel = strings.ToLower(strings.TrimSpace(p.Channel))
	if p.Channel == "" {
		p.Channel = "stable"
	}
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	res, err := db.DB.Exec(
		`INSERT INTO ota_packages(platform, version, channel, file_name, file_path, file_size, sha256, signature, release_notes, force_update, min_supported_version, downloads, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.Platform, p.Version, p.Channel, p.FileName, p.FilePath, p.FileSize, p.SHA256, p.Signature, p.ReleaseNotes,
		boolToInt(p.ForceUpdate), p.MinSupportedVersion, 0, now, now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	p.ID = id
	return nil
}

func UpdateOTA(p *OTAPackage) error {
	p.UpdatedAt = time.Now().Unix()
	_, err := db.DB.Exec(
		`UPDATE ota_packages SET version=?, channel=?, file_name=?, file_size=?, sha256=?, signature=?, release_notes=?, force_update=?, min_supported_version=?, revoked=?, updated_at=? WHERE id=?`,
		p.Version, p.Channel, p.FileName, p.FileSize, p.SHA256, p.Signature, p.ReleaseNotes,
		boolToInt(p.ForceUpdate), p.MinSupportedVersion, boolToInt(p.Revoked), p.UpdatedAt, p.ID,
	)
	return err
}

func DeleteOTA(id int64) error {
	_, err := db.DB.Exec(`UPDATE ota_packages SET revoked=1 WHERE id=?`, id)
	return err
}

func ListOTA(includeRevoked bool) ([]OTAPackage, error) {
	q := `SELECT id, platform, version, channel, file_name, file_size, sha256, COALESCE(signature,''), COALESCE(release_notes,''), force_update, COALESCE(min_supported_version,''), downloads, created_at, updated_at, revoked FROM ota_packages`
	if !includeRevoked {
		q += ` WHERE revoked=0`
	}
	q += ` ORDER BY id DESC LIMIT 500`
	rows, err := db.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OTAPackage{}
	for rows.Next() {
		var p OTAPackage
		var fu, rev int
		if err := rows.Scan(&p.ID, &p.Platform, &p.Version, &p.Channel, &p.FileName, &p.FileSize, &p.SHA256, &p.Signature, &p.ReleaseNotes, &fu, &p.MinSupportedVersion, &p.Downloads, &p.CreatedAt, &p.UpdatedAt, &rev); err != nil {
			return nil, err
		}
		p.ForceUpdate = fu != 0
		p.Revoked = rev != 0
		out = append(out, p)
	}
	return out, nil
}

func GetLatestOTA(platform, currentVersion string) (*OTAPackage, error) {
	row := db.DB.QueryRow(
		`SELECT id, platform, version, channel, file_name, file_size, sha256, COALESCE(signature,''), COALESCE(release_notes,''), force_update, COALESCE(min_supported_version,''), downloads, created_at, updated_at, revoked FROM ota_packages WHERE platform=? AND revoked=0 AND channel='stable' ORDER BY id DESC LIMIT 1`,
		strings.ToLower(platform),
	)
	var p OTAPackage
	var fu, rev int
	if err := row.Scan(&p.ID, &p.Platform, &p.Version, &p.Channel, &p.FileName, &p.FileSize, &p.SHA256, &p.Signature, &p.ReleaseNotes, &fu, &p.MinSupportedVersion, &p.Downloads, &p.CreatedAt, &p.UpdatedAt, &rev); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.ForceUpdate = fu != 0
	p.Revoked = rev != 0
	return &p, nil
}

func GetOTAPackage(id int64) (*OTAPackage, string, error) {
	row := db.DB.QueryRow(
		`SELECT id, platform, version, channel, file_name, file_path, file_size, sha256, COALESCE(signature,''), COALESCE(release_notes,''), force_update, COALESCE(min_supported_version,''), downloads, created_at, updated_at, revoked FROM ota_packages WHERE id=?`,
		id,
	)
	var p OTAPackage
	var fu, rev int
	if err := row.Scan(&p.ID, &p.Platform, &p.Version, &p.Channel, &p.FileName, &p.FilePath, &p.FileSize, &p.SHA256, &p.Signature, &p.ReleaseNotes, &fu, &p.MinSupportedVersion, &p.Downloads, &p.CreatedAt, &p.UpdatedAt, &rev); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", errors.New("package not found")
		}
		return nil, "", err
	}
	p.ForceUpdate = fu != 0
	p.Revoked = rev != 0
	return &p, p.FilePath, nil
}

func IncrementOTADownloads(id int64) {
	_, _ = db.DB.Exec(`UPDATE ota_packages SET downloads=downloads+1 WHERE id=?`, id)
}


package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/linkall/server/internal/db"
)

type Announcement struct {
	ID         int64  `json:"id"`
	AuthorID   int64  `json:"author_id"`
	AuthorName string `json:"author_name,omitempty"`
	Title      string `json:"title"`
	ContentMD  string `json:"content_md"`
	Platform   string `json:"platform"`
	MinVersion string `json:"min_version"`
	Pinned     bool   `json:"pinned"`
	ForceRead  bool   `json:"force_read"`
	Signature  string `json:"signature"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	Revoked    bool   `json:"revoked"`
}

func CreateAnnouncement(a *Announcement) error {
	if a.Title == "" {
		return errors.New("标题必填")
	}
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	res, err := db.DB.Exec(
		`INSERT INTO announcements(author_id,title,content_md,platform,min_version,pinned,force_read,signature,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.AuthorID, a.Title, a.ContentMD, a.Platform, a.MinVersion,
		boolToInt(a.Pinned), boolToInt(a.ForceRead), a.Signature, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	return nil
}

func UpdateAnnouncement(a *Announcement) error {
	a.UpdatedAt = time.Now().Unix()
	_, err := db.DB.Exec(
		`UPDATE announcements SET title=?, content_md=?, platform=?, min_version=?, pinned=?, force_read=?, signature=?, updated_at=?, revoked=? WHERE id=?`,
		a.Title, a.ContentMD, a.Platform, a.MinVersion, boolToInt(a.Pinned), boolToInt(a.ForceRead),
		a.Signature, a.UpdatedAt, boolToInt(a.Revoked), a.ID,
	)
	return err
}

func DeleteAnnouncement(id int64) error {
	_, err := db.DB.Exec(`UPDATE announcements SET revoked=1 WHERE id=?`, id)
	return err
}

func ListAnnouncements(includeRevoked bool) ([]Announcement, error) {
	q := `SELECT a.id, a.author_id, COALESCE(u.username,''), a.title, a.content_md, COALESCE(a.platform,''), COALESCE(a.min_version,''), a.pinned, a.force_read, COALESCE(a.signature,''), a.created_at, a.updated_at, a.revoked FROM announcements a LEFT JOIN users u ON u.id=a.author_id`
	if !includeRevoked {
		q += ` WHERE a.revoked=0`
	}
	q += ` ORDER BY a.pinned DESC, a.id DESC LIMIT 500`
	rows, err := db.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Announcement{}
	for rows.Next() {
		var a Announcement
		var pin, fr, rev int
		if err := rows.Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.ContentMD, &a.Platform, &a.MinVersion, &pin, &fr, &a.Signature, &a.CreatedAt, &a.UpdatedAt, &rev); err != nil {
			return nil, err
		}
		a.Pinned = pin != 0
		a.ForceRead = fr != 0
		a.Revoked = rev != 0
		out = append(out, a)
	}
	return out, nil
}

func MarkAnnouncementRead(annID, userID int64) error {
	_, err := db.DB.Exec(
		`INSERT OR IGNORE INTO announcement_reads(announcement_id, user_id, read_at) VALUES(?,?,?)`,
		annID, userID, time.Now().Unix(),
	)
	return err
}

func ListUnreadAnnouncements(userID int64) ([]Announcement, error) {
	rows, err := db.DB.Query(
		`SELECT a.id, a.author_id, COALESCE(u.username,''), a.title, a.content_md, COALESCE(a.platform,''), COALESCE(a.min_version,''), a.pinned, a.force_read, COALESCE(a.signature,''), a.created_at, a.updated_at, a.revoked
		 FROM announcements a LEFT JOIN users u ON u.id=a.author_id
		 LEFT JOIN announcement_reads r ON r.announcement_id=a.id AND r.user_id=?
		 WHERE a.revoked=0 AND r.user_id IS NULL
		 ORDER BY a.pinned DESC, a.id DESC LIMIT 100`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Announcement{}
	for rows.Next() {
		var a Announcement
		var pin, fr, rev int
		if err := rows.Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.ContentMD, &a.Platform, &a.MinVersion, &pin, &fr, &a.Signature, &a.CreatedAt, &a.UpdatedAt, &rev); err != nil {
			return nil, err
		}
		a.Pinned = pin != 0
		a.ForceRead = fr != 0
		a.Revoked = rev != 0
		out = append(out, a)
	}
	return out, nil
}

func GetAnnouncement(id int64) (*Announcement, error) {
	row := db.DB.QueryRow(
		`SELECT a.id, a.author_id, COALESCE(u.username,''), a.title, a.content_md, COALESCE(a.platform,''), COALESCE(a.min_version,''), a.pinned, a.force_read, COALESCE(a.signature,''), a.created_at, a.updated_at, a.revoked FROM announcements a LEFT JOIN users u ON u.id=a.author_id WHERE a.id=?`,
		id,
	)
	var a Announcement
	var pin, fr, rev int
	if err := row.Scan(&a.ID, &a.AuthorID, &a.AuthorName, &a.Title, &a.ContentMD, &a.Platform, &a.MinVersion, &pin, &fr, &a.Signature, &a.CreatedAt, &a.UpdatedAt, &rev); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("公告不存在")
		}
		return nil, err
	}
	a.Pinned = pin != 0
	a.ForceRead = fr != 0
	a.Revoked = rev != 0
	return &a, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

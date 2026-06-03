package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/linkall/server/internal/db"
)

const (
	charsetNum      = "0123456789"
	charsetAlphaNum = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

func randomString(n int, charset string) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}

// GenerateDeviceCode 12 位大写字母+数字（去除易混字符）
func GenerateDeviceCode() (string, error) {
	return randomString(12, charsetAlphaNum)
}

// GenerateNumericCode 6 位数字邀请码
func GenerateNumericCode() (string, error) {
	return randomString(8, charsetNum)
}

type Invite struct {
	ID        int64
	Code      string
	CreatedBy int64
	MaxUses   int
	UsedCount int
	TTLHours  int
	ExpiresAt int64
	Revoked   bool
	Note      string
}

func CreateInvite(creator int64, maxUses, ttlHrs int, note string) (*Invite, error) {
	if maxUses <= 0 {
		maxUses = 1
	}
	if ttlHrs <= 0 {
		ttlHrs = 72
	}
	code, err := GenerateNumericCode()
	if err != nil {
		return nil, err
	}
	now := db.Now()
	exp := time.Now().Add(time.Duration(ttlHrs) * time.Hour).Unix()
	res, err := db.DB.Exec(
		`INSERT INTO invites(code, created_by, max_uses, used_count, ttl_hours, created_at, expires_at, note) VALUES(?,?,?,?,?,?,?,?)`,
		code, creator, maxUses, 0, ttlHrs, now, exp, note,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Invite{ID: id, Code: code, CreatedBy: creator, MaxUses: maxUses, TTLHours: ttlHrs, ExpiresAt: exp, Note: note}, nil
}

func ConsumeInvite(code string, userID int64) error {
	code = strings.TrimSpace(code)
	row := db.DB.QueryRow(`SELECT id, max_uses, used_count, expires_at, revoked FROM invites WHERE code=?`, code)
	var id, max, used, exp, rev int64
	if err := row.Scan(&id, &max, &used, &exp, &rev); err != nil {
		return errors.New("邀请码无效")
	}
	if rev != 0 {
		return errors.New("邀请码已吊销")
	}
	if time.Now().Unix() > exp {
		return errors.New("邀请码已过期")
	}
	if used >= max {
		return errors.New("邀请码已被使用完")
	}
	_, err := db.DB.Exec(
		`UPDATE invites SET used_count=used_count+1, used_by=COALESCE(used_by,?) WHERE id=? AND used_count<?`,
		userID, id, max,
	)
	return err
}

func RevokeInvite(id, by int64) error {
	_, err := db.DB.Exec(`UPDATE invites SET revoked=1 WHERE id=?`, id)
	return err
}

func ListInvites() ([]Invite, error) {
	rows, err := db.DB.Query(`SELECT id, code, created_by, max_uses, used_count, ttl_hours, expires_at, revoked, COALESCE(note,'') FROM invites ORDER BY id DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invite{}
	for rows.Next() {
		var it Invite
		var rev int
		if err := rows.Scan(&it.ID, &it.Code, &it.CreatedBy, &it.MaxUses, &it.UsedCount, &it.TTLHours, &it.ExpiresAt, &rev, &it.Note); err != nil {
			return nil, err
		}
		it.Revoked = rev != 0
		out = append(out, it)
	}
	return out, nil
}

func fmtInvite(s string) string { return s }

var _ = fmtInvite

package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/linkall/server/internal/db"
)

type User struct {
	ID            int64
	Username      string
	IsAdmin       bool
	IsSuperAdmin  bool
	Banned        bool
	CreatedAt     int64
	LastLoginIP   string
	LastLoginAt   int64
	Locale        string
	Avatar        string
}

func CreateUser(username, password string, isAdmin, isSuper bool) (*User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("用户名长度需 3-32")
	}
	if len(password) < 6 {
		return nil, errors.New("密码至少 6 位")
	}
	ph, err := HashPassword(password, DefaultParams())
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	res, err := db.DB.Exec(
		`INSERT INTO users(username, password_hash, is_admin, is_super_admin, created_at, locale) VALUES(?,?,?,?,?,?)`,
		username, ph, boolToInt(isAdmin), boolToInt(isSuper), now, "zh-CN",
	)
	if err != nil {
		return nil, errors.New("用户已存在或写入失败")
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, IsAdmin: isAdmin, IsSuperAdmin: isSuper, CreatedAt: now, Locale: "zh-CN"}, nil
}

func FindUserByName(username string) (*User, string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	row := db.DB.QueryRow(
		`SELECT id, username, password_hash, is_admin, is_super_admin, banned, created_at, COALESCE(last_login_ip,''), COALESCE(last_login_at,0), COALESCE(locale,'zh-CN'), COALESCE(avatar,'') FROM users WHERE username=?`,
		username,
	)
	var u User
	var ph string
	var adm, sa, ban int
	if err := row.Scan(&u.ID, &u.Username, &ph, &adm, &sa, &ban, &u.CreatedAt, &u.LastLoginIP, &u.LastLoginAt, &u.Locale, &u.Avatar); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errors.New("用户不存在")
		}
		return nil, "", err
	}
	u.IsAdmin = adm != 0
	u.IsSuperAdmin = sa != 0
	u.Banned = ban != 0
	return &u, ph, nil
}

func GetUser(id int64) (*User, error) {
	row := db.DB.QueryRow(
		`SELECT id, username, is_admin, is_super_admin, banned, created_at, COALESCE(last_login_ip,''), COALESCE(last_login_at,0), COALESCE(locale,'zh-CN'), COALESCE(avatar,'') FROM users WHERE id=?`,
		id,
	)
	var u User
	var adm, sa, ban int
	if err := row.Scan(&u.ID, &u.Username, &adm, &sa, &ban, &u.CreatedAt, &u.LastLoginIP, &u.LastLoginAt, &u.Locale, &u.Avatar); err != nil {
		return nil, err
	}
	u.IsAdmin = adm != 0
	u.IsSuperAdmin = sa != 0
	u.Banned = ban != 0
	return &u, nil
}

func VerifyLogin(username, password string) (*User, error) {
	u, ph, err := FindUserByName(username)
	if err != nil {
		return nil, err
	}
	if u.Banned {
		return nil, errors.New("账户已被封禁")
	}
	ok, err := VerifyPassword(password, ph, DefaultParams())
	if err != nil || !ok {
		return nil, errors.New("用户名或密码错误")
	}
	return u, nil
}

func UpdatePassword(userID int64, oldPw, newPw string) error {
	u, ph, err := FindUserByName(getUsername(userID))
	if err != nil {
		return err
	}
	if u.ID != userID {
		return errors.New("用户不一致")
	}
	ok, err := VerifyPassword(oldPw, ph, DefaultParams())
	if err != nil || !ok {
		return errors.New("原密码错误")
	}
	if len(newPw) < 6 {
		return errors.New("新密码至少 6 位")
	}
	nph, err := HashPassword(newPw, DefaultParams())
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?`, nph, userID)
	return err
}

func getUsername(id int64) string {
	var n string
	_ = db.DB.QueryRow(`SELECT username FROM users WHERE id=?`, id).Scan(&n)
	return n
}

func RecordLogin(userID int64, ip string) {
	_, _ = db.DB.Exec(`UPDATE users SET last_login_ip=?, last_login_at=? WHERE id=?`, ip, time.Now().Unix(), userID)
	_, _ = db.DB.Exec(`INSERT INTO audit_logs(actor_id, action, ip, created_at) VALUES(?,?,?,?)`,
		userID, "login", ip, time.Now().Unix())
}

func SetLocale(userID int64, locale string) error {
	_, err := db.DB.Exec(`UPDATE users SET locale=? WHERE id=?`, locale, userID)
	return err
}

func BanUser(userID int64, ban bool) error {
	v := 0
	if ban {
		v = 1
	}
	_, err := db.DB.Exec(`UPDATE users SET banned=? WHERE id=?`, v, userID)
	return err
}

func SetAdmin(userID int64, admin, super bool) error {
	_, err := db.DB.Exec(`UPDATE users SET is_admin=?, is_super_admin=? WHERE id=?`,
		boolToInt(admin), boolToInt(super), userID)
	return err
}

func ListUsers() ([]User, error) {
	rows, err := db.DB.Query(
		`SELECT id, username, is_admin, is_super_admin, banned, created_at, COALESCE(last_login_ip,''), COALESCE(last_login_at,0), COALESCE(locale,'zh-CN'), COALESCE(avatar,'') FROM users ORDER BY id DESC LIMIT 1000`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		var adm, sa, ban int
		if err := rows.Scan(&u.ID, &u.Username, &adm, &sa, &ban, &u.CreatedAt, &u.LastLoginIP, &u.LastLoginAt, &u.Locale, &u.Avatar); err != nil {
			return nil, err
		}
		u.IsAdmin = adm != 0
		u.IsSuperAdmin = sa != 0
		u.Banned = ban != 0
		out = append(out, u)
	}
	return out, nil
}

func CountUsers(ctx context.Context) (int, error) {
	var n int
	err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func FindUserByNameByID(id int64) (string, string, error) {
	row := db.DB.QueryRow(`SELECT username, password_hash FROM users WHERE id=?`, id)
	var u, ph string
	if err := row.Scan(&u, &ph); err != nil {
		return "", "", err
	}
	return u, ph, nil
}

func AdminResetPassword(userID int64, newPw string) error {
	if len(newPw) < 6 {
		return errors.New("新密码至少 6 位")
	}
	ph, err := HashPassword(newPw, DefaultParams())
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?`, ph, userID)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

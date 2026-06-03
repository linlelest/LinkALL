// 账号锁定：连续 N 次失败后锁定 X 分钟
package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/linkall/server/internal/db"
)

type LockoutCfg struct {
	MaxAttempts int
	LockoutMins int
}

func DefaultLockoutCfg() LockoutCfg {
	return LockoutCfg{MaxAttempts: 5, LockoutMins: 15}
}

// 解析器：从 settings 表读
func LoadLockoutCfg() LockoutCfg {
	cfg := DefaultLockoutCfg()
	var a, m string
	if err := db.DB.QueryRow(`SELECT v FROM settings WHERE k='lockout_attempts'`).Scan(&a); err == nil {
		var n int
		_, _ = parseInt(a, &n)
		if n > 0 {
			cfg.MaxAttempts = n
		}
	}
	if err := db.DB.QueryRow(`SELECT v FROM settings WHERE k='lockout_mins'`).Scan(&m); err == nil {
		var n int
		_, _ = parseInt(m, &n)
		if n > 0 {
			cfg.LockoutMins = n
		}
	}
	return cfg
}

func SetLockoutCfg(cfg LockoutCfg) {
	_ = upsertSetting("lockout_attempts", itoa(cfg.MaxAttempts))
	_ = upsertSetting("lockout_mins", itoa(cfg.LockoutMins))
}

type LockState struct {
	Failed   int
	LockedAt int64
}

func GetLockState(username string) (LockState, error) {
	var s LockState
	err := db.DB.QueryRow(
		`SELECT COALESCE(failed_attempts,0), COALESCE(locked_at,0) FROM login_attempts WHERE username=?`,
		username,
	).Scan(&s.Failed, &s.LockedAt)
	if err == sql.ErrNoRows {
		return LockState{}, nil
	}
	return s, err
}

func IsLocked(username string, cfg LockoutCfg) (bool, time.Duration) {
	st, _ := GetLockState(username)
	if st.LockedAt == 0 {
		return false, 0
	}
	until := time.Unix(st.LockedAt, 0).Add(time.Duration(cfg.LockoutMins) * time.Minute)
	if time.Now().Before(until) {
		return true, time.Until(until)
	}
	// 过期自动解锁
	_ = ClearLockState(username)
	return false, 0
}

func RecordFailure(username, ip string) {
	_, _ = db.DB.Exec(
		`INSERT INTO login_attempts(username, failed_attempts, last_fail_at, last_ip)
		 VALUES(?,1,?,?)
		 ON CONFLICT(username) DO UPDATE SET
			failed_attempts=failed_attempts+1, last_fail_at=excluded.last_fail_at, last_ip=excluded.last_ip`,
		username, time.Now().Unix(), ip,
	)
}

func ClearLockState(username string) error {
	_, err := db.DB.Exec(`DELETE FROM login_attempts WHERE username=?`, username)
	return err
}

func LockNow(username string) error {
	_, err := db.DB.Exec(
		`UPDATE login_attempts SET locked_at=? WHERE username=?`,
		time.Now().Unix(), username,
	)
	return err
}

// CheckAndLock 失败计数到上限后锁定
func CheckAndLock(username, ip string, cfg LockoutCfg) (bool, error) {
	st, _ := GetLockState(username)
	st.Failed++
	if st.Failed >= cfg.MaxAttempts {
		_ = LockNow(username)
		return true, errors.New("账户已锁定，请稍后再试")
	}
	RecordFailure(username, ip)
	return false, nil
}

func upsertSetting(k, v string) error {
	_, err := db.DB.Exec(
		`INSERT INTO settings(k, v, updated_at) VALUES(?,?,?)
		 ON CONFLICT(k) DO UPDATE SET v=excluded.v, updated_at=excluded.updated_at`,
		k, v, time.Now().Unix(),
	)
	return err
}

func parseInt(s string, out *int) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(ch-'0')
	}
	*out = n
	return n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

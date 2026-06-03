// JWT 多密钥管理（kid）
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/linkall/server/internal/db"
)

type KeyManager struct {
	mu    sync.RWMutex
	cache map[string][]byte // kid -> secret
	active string           // 当前签名 kid
}

var KeyMgr = &KeyManager{cache: map[string][]byte{}}

func (k *KeyManager) Init() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	// 从 DB 读
	rows, err := db.DB.Query(`SELECT kid, secret, active FROM jwt_keys`)
	if err != nil {
		return err
	}
	defer rows.Close()
	k.cache = map[string][]byte{}
	var activeKid string
	for rows.Next() {
		var kid, secret string
		var active int
		if err := rows.Scan(&kid, &secret, &active); err != nil {
			return err
		}
		raw, _ := base64.StdEncoding.DecodeString(secret)
		if len(raw) < 32 {
			raw = []byte(secret) // 兼容明文
		}
		k.cache[kid] = raw
		if active == 1 {
			activeKid = kid
		}
	}
	// 没有任何密钥：自动生成
	if len(k.cache) == 0 {
		kid, secret, err := generateKey()
		if err != nil {
			return err
		}
		_, _ = db.DB.Exec(
			`INSERT INTO jwt_keys(kid, secret, created_at, active) VALUES(?,?,?,1)`,
			kid, base64.StdEncoding.EncodeToString(secret), time.Now().Unix(),
		)
		k.cache[kid] = secret
		activeKid = kid
	}
	// 没有 active：选第一个
	if activeKid == "" {
		for kid := range k.cache {
			activeKid = kid
			_, _ = db.DB.Exec(`UPDATE jwt_keys SET active=1 WHERE kid=?`, kid)
			break
		}
	}
	k.active = activeKid
	return nil
}

func (k *KeyManager) ActiveKid() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.active
}

func (k *KeyManager) ActiveSecret() []byte {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cache[k.active]
}

func (k *KeyManager) SecretByKid(kid string) ([]byte, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	s, ok := k.cache[kid]
	return s, ok
}

func (k *KeyManager) ListKeys() []map[string]any {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := []map[string]any{}
	for kid := range k.cache {
		out = append(out, map[string]any{
			"kid": kid, "active": kid == k.active,
		})
	}
	return out
}

func (k *KeyManager) AddKey() (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	kid, secret, err := generateKey()
	if err != nil {
		return "", err
	}
	_, err = db.DB.Exec(
		`INSERT INTO jwt_keys(kid, secret, created_at, active) VALUES(?,?,?,0)`,
		kid, base64.StdEncoding.EncodeToString(secret), time.Now().Unix(),
	)
	if err != nil {
		return "", err
	}
	k.cache[kid] = secret
	return kid, nil
}

// Rotate 生成新密钥并设为 active；老密钥保留以验签存量 token
func (k *KeyManager) Rotate() (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	kid, secret, err := generateKey()
	if err != nil {
		return "", err
	}
	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	_, _ = tx.Exec(`UPDATE jwt_keys SET active=0`)
	_, err = tx.Exec(
		`INSERT INTO jwt_keys(kid, secret, created_at, active) VALUES(?,?,?,1)`,
		kid, base64.StdEncoding.EncodeToString(secret), time.Now().Unix(),
	)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}
	_ = tx.Commit()
	// 替换缓存
	for kk := range k.cache {
		if kk == k.active {
			// 保留旧 kid，验签用；不删
		}
	}
	k.cache[kid] = secret
	k.active = kid
	return kid, nil
}

// Revoke 删除某 kid
func (k *KeyManager) Revoke(kid string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if kid == k.active {
		return errors.New("cannot revoke active key")
	}
	_, err := db.DB.Exec(`DELETE FROM jwt_keys WHERE kid=?`, kid)
	if err != nil {
		return err
	}
	delete(k.cache, kid)
	return nil
}

func generateKey() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	// kid = 时间戳 + 6 位随机（base32 短串）
	tb := time.Now().Unix()
	rb := make([]byte, 3)
	_, _ = rand.Read(rb)
	rid := base32short(rb)
	kid := itoa64(tb) + "-" + rid
	return kid, b, nil
}

func itoa64(n int64) string {
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

func base32short(b []byte) string {
	const alpha = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	out := make([]byte, 0, len(b)*2)
	for _, x := range b {
		out = append(out, alpha[int(x)%len(alpha)])
	}
	return string(out)
}

// ===== 包装旧的 JWT 解析 / 签名（向后兼容单密钥） =====

type Claims struct {
	UserID    int64  `json:"uid"`
	Username  string `json:"u"`
	Admin     bool   `json:"a"`
	SuperAdm  bool   `json:"sa"`
	TokenType string `json:"tt"`
	DeviceID  int64  `json:"did,omitempty"`
	KID       string `json:"kid,omitempty"`
	jwt.RegisteredClaims
}

func NewJWTFromEnv() *JWT {
	secret := []byte(strings.TrimSpace(db.GetSetting("jwt_active_secret_fallback")))
	if len(secret) < 16 {
		secret = KeyMgr.ActiveSecret()
	}
	ttl := int(db.GetSettingInt("jwt_ttl_hours", 168))
	if ttl < 1 || ttl > 8760 {
		ttl = 168
	}
	return &JWT{Secret: secret, TTL: time.Duration(ttl) * time.Hour}
}

type JWT struct {
	Secret []byte
	TTL    time.Duration
}

func (j *JWT) Sign(c Claims) (string, error) {
	c.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(j.TTL))
	c.RegisteredClaims.IssuedAt = jwt.NewNumericDate(time.Now())
	c.RegisteredClaims.Issuer = "linkall-server"
	c.KID = KeyMgr.ActiveKid()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, &c)
	t.Header["kid"] = c.KID
	return t.SignedString(j.Secret)
}

func (j *JWT) Parse(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad signing method")
		}
		// 优先按 kid 选密钥
		if kidRaw, ok := t.Header["kid"]; ok {
			if kid, ok := kidRaw.(string); ok {
				if s, ok := KeyMgr.SecretByKid(kid); ok {
					return s, nil
				}
			}
		}
		// 否则用默认密钥
		return j.Secret, nil
	},
		jwt.WithIssuer("linkall-server"),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// 兼容 sqlite.ErrNoRows
var _ = sql.ErrNoRows

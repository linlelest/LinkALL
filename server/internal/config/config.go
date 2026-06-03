package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr  string
	PublicURL string
	TLSCert   string
	TLSKey    string

	DBPath string
	OTADir string
	LogDir string

	JWTSecret    string
	JWTTTLHours  int
	Argon2Time   uint32
	Argon2Memory uint32
	Argon2Thrs   uint8
	Argon2KeyLen uint32

	OfficialServer       string
	MaxConcurrentSess    int
	SessionIdleTimeoutM  int
	DataRetentionDays    int
	RequireDeviceCodeDef bool
	AllowAnonymousDef    bool
	InviteDefaultTTLHrs  int

	STUNURLs []string
	TURNURL  string
	TURNUser string
	TURNCred string
	// TURN 凭据模式：
	//   - TURNUser + TURNCred 非空 → 静态凭据（兼容老部署）
	//   - TURNSecret 非空 → coturn use-auth-secret 模式：username = "<expiry>:<userId>",
	//     credential = base64(HMAC-SHA1(SECRET, username))，TTL 默认 3600s
	TURNSecret      string
	TURNCredTTLSecs int

	// FCM (HTTP v1 API)
	FCMProjectID  string
	FCMOAuthToken string
}

var C *Config

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("[config] .env not loaded: %v (using env / defaults)", err)
	}
	c := &Config{
		HTTPAddr:             getStr("HTTP_ADDR", ":8080"),
		PublicURL:            getStr("PUBLIC_URL", "http://127.0.0.1:8080"),
		TLSCert:              getStr("TLS_CERT", ""),
		TLSKey:               getStr("TLS_KEY", ""),
		DBPath:               getStr("DB_PATH", "./data/linkall.db"),
		OTADir:               getStr("OTA_DIR", "./data/ota"),
		LogDir:               getStr("LOG_DIR", "./data/logs"),
		JWTSecret:            getStr("JWT_SECRET", "change-me-to-a-long-random-string-please-32+chars"),
		JWTTTLHours:          getInt("JWT_TTL_HOURS", 720),
		Argon2Time:           uint32(getInt("ARGON2_TIME", 2)),
		Argon2Memory:         uint32(getInt("ARGON2_MEMORY_KB", 65536)),
		Argon2Thrs:           uint8(getInt("ARGON2_THREADS", 2)),
		Argon2KeyLen:         uint32(getInt("ARGON2_KEYLEN", 32)),
		OfficialServer:       getStr("OFFICIAL_SERVER", "http://127.0.0.1:8080"),
		MaxConcurrentSess:    getInt("MAX_CONCURRENT_SESSIONS", 200),
		SessionIdleTimeoutM:  getInt("SESSION_IDLE_TIMEOUT_MIN", 30),
		DataRetentionDays:    getInt("DATA_RETENTION_DAYS", 30),
		RequireDeviceCodeDef: getBool("REQUIRE_DEVICE_CODE_DEFAULT", true),
		AllowAnonymousDef:    getBool("ALLOW_ANONYMOUS_DEFAULT", true),
		InviteDefaultTTLHrs:  getInt("INVITE_DEFAULT_TTL_HOURS", 72),
		STUNURLs:             splitCSV(getStr("STUN_URLS", "stun:stun.l.google.com:19302")),
		TURNURL:              getStr("TURN_URL", ""),
		TURNUser:             getStr("TURN_USER", ""),
		TURNCred:             getStr("TURN_CRED", ""),
		TURNSecret:           getStr("TURN_SECRET", ""),
		TURNCredTTLSecs:      getInt("TURN_CRED_TTL_SECS", 3600),
		FCMProjectID:         getStr("FCM_PROJECT_ID", ""),
		FCMOAuthToken:        getStr("FCM_OAUTH_TOKEN", ""),
	}
	C = c
	return c
}

func (c *Config) EnsureDirs() {
	for _, d := range []string{c.DBPath, c.OTADir, c.LogDir} {
		if d == "" {
			continue
		}
		dir := d
		if i := strings.LastIndex(dir, string(os.PathSeparator)); i > 0 {
			dir = dir[:i]
		}
		_ = os.MkdirAll(dir, 0o755)
	}
	_ = os.MkdirAll(c.OTADir, 0o755)
	_ = os.MkdirAll(c.LogDir, 0o755)
}

func getStr(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, ok := os.LookupEnv(k); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBool(k string, def bool) bool {
	if v, ok := os.LookupEnv(k); ok {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

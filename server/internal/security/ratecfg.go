// 限流配置：3 档可调，缺省 medium
package security

import (
	"strconv"
	"strings"
	"sync"

	"github.com/linkall/server/internal/db"
)

type Strictness string

const (
	StrictLoose   Strictness = "loose"
	StrictMedium  Strictness = "medium"
	StrictStrict  Strictness = "strict"
)

type RateConfig struct {
	Strictness       Strictness
	LoginPerWindow   int    // attempts
	LoginWindowSec   int
	RegisterPerWin   int
	RegisterWinSec   int
	DevRegPerWin     int
	DevRegWinSec     int
	WSConnectPerWin  int
	WSConnectWinSec  int
	WSCmdPerSec      int
	WSFilePerSec     int
	ReplayWindowSec  int
	WSMaxMessageKB   int
}

var (
	rcMu sync.RWMutex
	rc   = defaultRateConfig()
)

func defaultRateConfig() RateConfig {
	// 中等档默认值
	return RateConfig{
		Strictness:      StrictMedium,
		LoginPerWindow:  5,
		LoginWindowSec:  900,
		RegisterPerWin:  3,
		RegisterWinSec:  3600,
		DevRegPerWin:    5,
		DevRegWinSec:    3600,
		WSConnectPerWin: 30,
		WSConnectWinSec: 60,
		WSCmdPerSec:     30,
		WSFilePerSec:    60,
		ReplayWindowSec: 30,
		WSMaxMessageKB:  1024,
	}
}

func LoadRateConfig() RateConfig {
	rcMu.Lock()
	defer rcMu.Unlock()
	s := Strictness(strings.ToLower(strings.TrimSpace(db.GetSetting("rate_limit_strictness"))))
	if s == "" {
		s = StrictMedium
	}
	c := defaultRateConfig()
	c.Strictness = s
	switch s {
	case StrictLoose:
		c.LoginPerWindow, c.LoginWindowSec = 10, 900
		c.RegisterPerWin, c.RegisterWinSec = 5, 3600
		c.DevRegPerWin, c.DevRegWinSec = 10, 3600
		c.WSConnectPerWin, c.WSConnectWinSec = 60, 60
		c.WSCmdPerSec = 60
		c.WSFilePerSec = 120
		c.ReplayWindowSec = 60
		c.WSMaxMessageKB = 4096
	case StrictStrict:
		c.LoginPerWindow, c.LoginWindowSec = 3, 900
		c.RegisterPerWin, c.RegisterWinSec = 1, 3600
		c.DevRegPerWin, c.DevRegWinSec = 2, 3600
		c.WSConnectPerWin, c.WSConnectWinSec = 10, 60
		c.WSCmdPerSec = 10
		c.WSFilePerSec = 30
		c.ReplayWindowSec = 10
		c.WSMaxMessageKB = 512
	}
	// 允许 .env 显式覆盖
	if v := db.GetSetting("rate_limit_login"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.LoginPerWindow, c.LoginWindowSec = a, w
		}
	}
	if v := db.GetSetting("rate_limit_register"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.RegisterPerWin, c.RegisterWinSec = a, w
		}
	}
	if v := db.GetSetting("rate_limit_device_register"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.DevRegPerWin, c.DevRegWinSec = a, w
		}
	}
	if v := db.GetSetting("rate_limit_ws_connect"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.WSConnectPerWin, c.WSConnectWinSec = a, w
		}
	}
	if v := db.GetSetting("rate_limit_ws_cmd"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.WSCmdPerSec = a
			_ = w
		}
	}
	if v := db.GetSetting("rate_limit_ws_file"); v != "" {
		if a, w := parseKV(v); a > 0 && w > 0 {
			c.WSFilePerSec = a
			_ = w
		}
	}
	if v := db.GetSetting("ws_replay_window_sec"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			c.ReplayWindowSec = n
		}
	}
	if v := db.GetSetting("ws_max_message_kb"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			c.WSMaxMessageKB = n
		}
	}
	rc = c
	return c
}

func GetRateConfig() RateConfig {
	rcMu.RLock()
	defer rcMu.RUnlock()
	return rc
}

func SetRateConfig(c RateConfig) {
	rcMu.Lock()
	rc = c
	rcMu.Unlock()
	_ = db.SetSetting("rate_limit_strictness", string(c.Strictness))
	_ = db.SetSetting("rate_limit_login", joinKV(c.LoginPerWindow, c.LoginWindowSec))
	_ = db.SetSetting("rate_limit_register", joinKV(c.RegisterPerWin, c.RegisterWinSec))
	_ = db.SetSetting("rate_limit_device_register", joinKV(c.DevRegPerWin, c.DevRegWinSec))
	_ = db.SetSetting("rate_limit_ws_connect", joinKV(c.WSConnectPerWin, c.WSConnectWinSec))
	_ = db.SetSetting("rate_limit_ws_cmd", joinKV(c.WSCmdPerSec, 1))
	_ = db.SetSetting("rate_limit_ws_file", joinKV(c.WSFilePerSec, 1))
	_ = db.SetSetting("ws_replay_window_sec", strconv.Itoa(c.ReplayWindowSec))
	_ = db.SetSetting("ws_max_message_kb", strconv.Itoa(c.WSMaxMessageKB))
}

func parseKV(s string) (int, int) {
	parts := strings.SplitN(s, "|", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	a, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return a, b
}

func joinKV(a, b int) string {
	return strconv.Itoa(a) + "|" + strconv.Itoa(b)
}

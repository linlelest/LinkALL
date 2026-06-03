package security

import "sync"

var (
	onceLimiter sync.Once
	limiter     *Limiter
)

// GetLimiter 返回全局限流器单例
func GetLimiter() *Limiter {
	onceLimiter.Do(func() {
		limiter = NewLimiter()
		// 应用默认规则（实际值可由 LoadRateConfig 覆盖）
		limiter.SetRule("login", 5, 900)
		limiter.SetRule("register", 3, 3600)
		limiter.SetRule("device_register", 5, 3600)
		limiter.SetRule("ws_connect", 30, 60)
		limiter.SetRule("ws_cmd", 30, 1)
		limiter.SetRule("ws_file", 60, 1)
	})
	return limiter
}

// ReloadLimiter 用最新 RateConfig 重设规则
func ReloadLimiter(c RateConfig) {
	l := GetLimiter()
	l.SetRule("login", c.LoginPerWindow, c.LoginWindowSec)
	l.SetRule("register", c.RegisterPerWin, c.RegisterWinSec)
	l.SetRule("device_register", c.DevRegPerWin, c.DevRegWinSec)
	l.SetRule("ws_connect", c.WSConnectPerWin, c.WSConnectWinSec)
	l.SetRule("ws_cmd", c.WSCmdPerSec, 1)
	l.SetRule("ws_file", c.WSFilePerSec, 1)
}

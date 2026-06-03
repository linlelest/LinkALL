package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/db"
	"github.com/linkall/server/internal/ota"
	"github.com/linkall/server/internal/security"
	"github.com/linkall/server/internal/signaling"
)

type Deps struct {
	JWT *auth.JWT
	Hub *signaling.Hub
}

func NewRouter(d Deps) *fiber.App {
	cfg := security.GetRateConfig()
	// 加载并应用限流规则
	security.ReloadLimiter(cfg)

	app := fiber.New(fiber.Config{
		AppName:               "LinkALL Server",
		DisableStartupMessage: true,
	})
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${method} ${path} ${status} ${latency}\n",
	}))
	// CORS：默认拒绝跨域；通过 ALLOW_ORIGINS_CSV 配置允许
	allowOrigins := strings.Split(strings.TrimSpace(db.GetSetting("allow_origins_csv")), ",")
	allowOrigins = append(allowOrigins, "") // 显式不填表示不放开
	corsCfg := cors.Config{
		AllowOrigins:     strings.Join(filterNonEmpty(allowOrigins), ","),
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Content-Type,Authorization,X-Requested-With",
		AllowCredentials: false,
	}
	if corsCfg.AllowOrigins == "" {
		corsCfg.AllowOrigins = config.C.PublicURL // 默认仅允许服务端自身 origin
	}
	app.Use(cors.New(corsCfg))
	// 安全响应头
	app.Use(security.SecurityHeaders(security.HeaderConfig{HSTS: config.C.TLSCert != "" && config.C.TLSKey != ""}))
	// CSRF：只对非 GET 强制
	csrf := &security.CSRFMiddleware{AllowedOrigins: filterNonEmpty(allowOrigins)}
	app.Use(csrf.Handler())

	// ===== 公共端点 =====
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "version": "1.0.0", "server_time": config.C.PublicURL})
	})
	app.Get("/api/config", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"public_url":          config.C.PublicURL,
			"official_server":     config.C.OfficialServer,
			"max_sessions":        config.C.MaxConcurrentSess,
			"idle_timeout_min":    config.C.SessionIdleTimeoutM,
			"data_retention_days": config.C.DataRetentionDays,
			"invite_ttl_hours":    config.C.InviteDefaultTTLHrs,
			"ice_servers":         buildIceServers(),
			"ota_pubkey":          ota.S.PublicKeyB64(),
			"ota_keyid":           ota.S.KeyID(),
		})
	})
	// OTA 公开公钥（无需登录）
	app.Get("/api/ota/pubkey", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"keyid":  ota.S.KeyID(),
			"pubkey": ota.S.PublicKeyB64(),
			"algo":   "ed25519",
		})
	})

	// 客户端崩溃 / 日志上报（公开，按 IP 限流）
	crashLimiter := IpRateLimit("crash_report")
	app.Post("/api/crash", crashLimiter, crashHandler)
	app.Post("/api/log", crashLimiter, logReportHandler)
	// FCM token 上报（公开，设备鉴权在 body 字段里）
	app.Post("/api/devices/fcm-token", registerFCMToken(db.DB))

	// ===== Auth =====
	ah := NewAuthHandler()
	loginLimiter := IpRateLimit("login")
	registerLimiter := IpRateLimit("register")
	app.Post("/api/auth/login", loginLimiter, ah.Login)
	app.Get("/api/auth/me", JWTAuth(d.JWT), ah.Me)
	app.Post("/api/auth/logout", JWTAuth(d.JWT), ah.Logout)
	app.Post("/api/auth/password", JWTAuth(d.JWT), ah.ChangePassword)
	app.Post("/api/auth/locale", JWTAuth(d.JWT), ah.SetLocale)
	app.Post("/api/auth/register", registerLimiter, ah.Register)

	// ===== Devices =====
	dh := NewDeviceHandler()
	devRegLimiter := IpRateLimit("device_register")
	app.Post("/api/devices/register", devRegLimiter, dh.Register)
	app.Post("/api/devices/login", loginLimiter, dh.Login)
	app.Get("/api/devices", JWTAuth(d.JWT), dh.List)
	app.Get("/api/devices/:id", JWTAuth(d.JWT), dh.Get)
	app.Patch("/api/devices/:id", JWTAuth(d.JWT), dh.Update)
	app.Post("/api/devices/:id/reset-code", JWTAuth(d.JWT), dh.ResetCode)
	app.Delete("/api/devices/:id", JWTAuth(d.JWT), dh.Delete)

	// ===== Announcements =====
	anh := NewAnnounceHandler()
	app.Get("/api/announcements", JWTAuth(d.JWT), anh.List)
	app.Get("/api/announcements/unread", JWTAuth(d.JWT), anh.Unread)
	app.Get("/api/announcements/:id", JWTAuth(d.JWT), anh.Get)
	app.Post("/api/announcements/:id/read", JWTAuth(d.JWT), anh.MarkRead)
	app.Post("/api/announcements", JWTAuth(d.JWT), AdminOnly(), anh.Create)
	app.Patch("/api/announcements/:id", JWTAuth(d.JWT), AdminOnly(), anh.Update)
	app.Delete("/api/announcements/:id", JWTAuth(d.JWT), AdminOnly(), anh.Delete)

	// ===== OTA =====
	oh := NewOTAHandler()
	app.Get("/api/ota/list", JWTAuth(d.JWT), oh.List)
	app.Get("/api/ota/check", oh.Check)
	app.Get("/api/ota/download/:id", oh.Download)
	app.Post("/api/ota/upload", JWTAuth(d.JWT), AdminOnly(), oh.Upload)
	app.Patch("/api/ota/:id", JWTAuth(d.JWT), AdminOnly(), oh.Update)
	app.Delete("/api/ota/:id", JWTAuth(d.JWT), AdminOnly(), oh.Delete)

	// 邀请码
	app.Get("/api/invites", JWTAuth(d.JWT), AdminOnly(), oh.ListInvites)
	app.Post("/api/invites", JWTAuth(d.JWT), AdminOnly(), oh.CreateInvite)
	app.Delete("/api/invites/:id", JWTAuth(d.JWT), AdminOnly(), oh.RevokeInvite)

	// ===== Admin =====
	admH := NewAdminHandler()
	app.Get("/api/admin/stats", JWTAuth(d.JWT), AdminOnly(), admH.Stats)
	app.Get("/api/admin/users", JWTAuth(d.JWT), AdminOnly(), admH.ListUsers)
	app.Patch("/api/admin/users/:id", JWTAuth(d.JWT), AdminOnly(), admH.UpdateUser)
	app.Delete("/api/admin/users/:id", JWTAuth(d.JWT), SuperAdminOnly(), admH.DeleteUser)
	// 限流配置
	app.Get("/api/admin/rate-limit", JWTAuth(d.JWT), AdminOnly(), admH.GetRateLimit)
	app.Put("/api/admin/rate-limit", JWTAuth(d.JWT), AdminOnly(), admH.SetRateLimit)
	// 审计日志
	app.Get("/api/admin/audit-logs", JWTAuth(d.JWT), AdminOnly(), admH.ListAudit)
	// 锁屏 / 解锁
	app.Get("/api/admin/lockout-cfg", JWTAuth(d.JWT), AdminOnly(), admH.GetLockoutCfg)
	app.Put("/api/admin/lockout-cfg", JWTAuth(d.JWT), AdminOnly(), admH.SetLockoutCfg)
	app.Post("/api/admin/unlock/:username", JWTAuth(d.JWT), AdminOnly(), admH.Unlock)
	// JWT 密钥管理
	app.Get("/api/admin/jwt-keys", JWTAuth(d.JWT), SuperAdminOnly(), admH.ListJWTKeys)
	app.Post("/api/admin/jwt-rotate", JWTAuth(d.JWT), SuperAdminOnly(), admH.RotateJWT)
	app.Delete("/api/admin/jwt-keys/:kid", JWTAuth(d.JWT), SuperAdminOnly(), admH.RevokeJWT)
	// OTA 密钥管理
	app.Get("/api/admin/ota-pubkey", JWTAuth(d.JWT), AdminOnly(), admH.OTAPubKey)
	app.Post("/api/admin/ota-rotate", JWTAuth(d.JWT), SuperAdminOnly(), admH.OTARotate)
	// 文件传输列表
	app.Get("/api/admin/file-transfers", JWTAuth(d.JWT), AdminOnly(), admH.ListFileTransfers)
	app.Get("/api/admin/crash-logs", JWTAuth(d.JWT), AdminOnly(), adminListCrashLogs)
	app.Get("/api/admin/crash-logs/stats", JWTAuth(d.JWT), AdminOnly(), adminCrashStats)
	// 唤醒 & FCM tokens
	app.Post("/api/admin/wake", JWTAuth(d.JWT), AdminOnly(), adminWakeDevice(db.DB))
	app.Get("/api/admin/fcm-tokens", JWTAuth(d.JWT), AdminOnly(), adminListFCMTokens(db.DB))
	// 重启设备（通过 WebSocket 信令下发重启指令）
	app.Post("/api/admin/restart-device/:deviceCode", JWTAuth(d.JWT), AdminOnly(), restartDevice(d.Hub))

	// ===== WebSocket 信令 =====
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/signaling", websocket.New(func(conn *websocket.Conn) {
		_ = d.Hub.Start(conn)
	}))

	return app
}

func buildIceServers() []fiber.Map {
	out := []fiber.Map{}
	for _, u := range config.C.STUNURLs {
		out = append(out, fiber.Map{"urls": u})
	}
	if config.C.TURNURL != "" {
		t := fiber.Map{"urls": config.C.TURNURL}
		// 优先 coturn use-auth-secret 模式（短时凭据）
		if config.C.TURNSecret != "" {
			username, credential := generateTurnCred(config.C.TURNSecret, config.C.TURNCredTTLSecs)
			t["username"] = username
			t["credential"] = credential
		} else {
			t["username"] = config.C.TURNUser
			t["credential"] = config.C.TURNCred
		}
		out = append(out, t)
	}
	return out
}

// generateTurnCred 按 coturn use-auth-secret 协议生成 (username, credential)
// username = "<unix_expiry>:<rand>"，credential = base64(HMAC-SHA1(secret, username))
// TTL 默认 3600s，到期后 coturn 自动拒绝
func generateTurnCred(secret string, ttlSecs int) (string, string) {
	if ttlSecs <= 0 {
		ttlSecs = 3600
	}
	expiry := time.Now().Unix() + int64(ttlSecs)
	// 8 字节随机 user-id 部分
	uid := make([]byte, 8)
	_, _ = rand.Read(uid)
	username := fmt.Sprintf("%d:%s", expiry, hex.EncodeToString(uid))
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, credential
}

func restartDevice(hub *signaling.Hub) fiber.Handler {
	return func(c *fiber.Ctx) error {
		code := strings.ToUpper(c.Params("deviceCode"))
		peer := hub.FindByCode(code)
		if peer == nil {
			return fiber.NewError(fiber.StatusNotFound, "device offline")
		}
		data, _ := json.Marshal(map[string]any{"op": "restart"})
		peer.SendEnvelope(&signaling.Envelope{
			Type: signaling.MsgCmd,
			Data: json.RawMessage(data),
			Ts:   time.Now().UnixMilli(),
		})
		return c.JSON(fiber.Map{"ok": true, "device_code": code})
	}
}

func filterNonEmpty(in []string) []string {
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

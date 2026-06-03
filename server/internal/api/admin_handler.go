package api

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/db"
	"github.com/linkall/server/internal/models"
	"github.com/linkall/server/internal/ota"
	"github.com/linkall/server/internal/security"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler { return &AdminHandler{} }

func (h *AdminHandler) Stats(c *fiber.Ctx) error {
	uc, _ := auth.CountUsers(c.Context())
	dc, _ := models.CountDevices()
	oc, _ := models.CountOnlineDevices()
	var sessions int
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM device_sessions WHERE closed=0`).Scan(&sessions)
	var sessionsTotal int64
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM device_sessions`).Scan(&sessionsTotal)
	var bytesTx, bytesRx int64
	_ = db.DB.QueryRow(`SELECT COALESCE(SUM(bytes_tx),0), COALESCE(SUM(bytes_rx),0) FROM device_sessions`).Scan(&bytesTx, &bytesRx)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return c.JSON(fiber.Map{
		"users":          uc,
		"devices":        dc,
		"online":         oc,
		"sessions":       sessions,
		"sessions_total": sessionsTotal,
		"bytes_tx":       bytesTx,
		"bytes_rx":       bytesRx,
		"server_time":    time.Now().Unix(),
		"go_version":     runtime.Version(),
		"go_routines":    runtime.NumGoroutine(),
		"go_mem_alloc":   mem.Alloc,
		"go_sys":         mem.Sys,
		"go_heap_inuse":  mem.HeapInuse,
		"ota_pubkey":     ota.S.PublicKeyB64(),
		"ota_keyid":      ota.S.KeyID(),
	})
}

func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	us, err := auth.ListUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(us)
}

type userActionReq struct {
	Ban         *bool  `json:"banned"`
	IsAdmin     *bool  `json:"is_admin"`
	IsSuper     *bool  `json:"is_super_admin"`
	NewPassword string `json:"new_password"`
}

func (h *AdminHandler) UpdateUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	var req userActionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.Ban != nil {
		_ = auth.BanUser(int64(id), *req.Ban)
	}
	if req.IsAdmin != nil {
		isSuper := false
		if req.IsSuper != nil {
			isSuper = *req.IsSuper
		}
		_ = auth.SetAdmin(int64(id), *req.IsAdmin, isSuper)
	} else if req.IsSuper != nil {
		_ = auth.SetAdmin(int64(id), false, *req.IsSuper)
	}
	if req.NewPassword != "" {
		if err := auth.AdminResetPassword(int64(id), req.NewPassword); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "user_update", IP: c.IP(), Target: "user:" + c.Params("id"), Detail: reqAsString(req)})
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	sa, _ := c.Locals(CtxIsSuper).(bool)
	if !sa {
		return c.Status(403).JSON(fiber.Map{"error": "需要超级管理员"})
	}
	if _, err := db.DB.Exec(`DELETE FROM users WHERE id=?`, id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "user_delete", IP: c.IP(), Target: "user:" + c.Params("id")})
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) GetRateLimit(c *fiber.Ctx) error {
	return c.JSON(security.LoadRateConfig())
}

func (h *AdminHandler) SetRateLimit(c *fiber.Ctx) error {
	var cfg security.RateConfig
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	// 默认值兜底
	if cfg.Strictness == "" {
		cfg.Strictness = security.StrictMedium
	}
	if cfg.LoginPerWindow <= 0 {
		cfg.LoginPerWindow = 5
	}
	if cfg.WSMaxMessageKB <= 0 {
		cfg.WSMaxMessageKB = 1024
	}
	security.SetRateConfig(cfg)
	security.ReloadLimiter(cfg)
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "rate_limit_change", IP: c.IP(), Detail: string(cfg.Strictness)})
	return c.JSON(fiber.Map{"ok": true, "config": cfg})
}

func (h *AdminHandler) ListAudit(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 200)
	logs, err := security.List(limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(logs)
}

func (h *AdminHandler) GetLockoutCfg(c *fiber.Ctx) error {
	return c.JSON(auth.LoadLockoutCfg())
}

func (h *AdminHandler) SetLockoutCfg(c *fiber.Ctx) error {
	var cfg auth.LockoutCfg
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.LockoutMins <= 0 {
		cfg.LockoutMins = 15
	}
	auth.SetLockoutCfg(cfg)
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "lockout_cfg_change", IP: c.IP()})
	return c.JSON(fiber.Map{"ok": true, "config": cfg})
}

func (h *AdminHandler) Unlock(c *fiber.Ctx) error {
	uname := c.Params("username")
	if uname == "" {
		return c.Status(400).JSON(fiber.Map{"error": "username 必填"})
	}
	_ = auth.ClearLockState(uname)
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "user_unlock", IP: c.IP(), Target: uname})
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) ListJWTKeys(c *fiber.Ctx) error {
	return c.JSON(auth.KeyMgr.ListKeys())
}

func (h *AdminHandler) RotateJWT(c *fiber.Ctx) error {
	kid, err := auth.KeyMgr.Rotate()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "jwt_rotate", IP: c.IP(), Target: kid})
	return c.JSON(fiber.Map{"ok": true, "kid": kid})
}

func (h *AdminHandler) RevokeJWT(c *fiber.Ctx) error {
	kid := c.Params("kid")
	if err := auth.KeyMgr.Revoke(kid); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "jwt_revoke", IP: c.IP(), Target: kid})
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AdminHandler) OTAPubKey(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"keyid":  ota.S.KeyID(),
		"pubkey": ota.S.PublicKeyB64(),
	})
}

func (h *AdminHandler) OTARotate(c *fiber.Ctx) error {
	if err := ota.S.Rotate(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "ota_key_rotate", IP: c.IP()})
	return c.JSON(fiber.Map{"ok": true, "keyid": ota.S.KeyID(), "pubkey": ota.S.PublicKeyB64()})
}

func (h *AdminHandler) ListFileTransfers(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	scope := c.Query("scope")
	key := c.Query("key")
	fts, err := models.ListFileTransfers(scope, key, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fts)
}

func reqAsString(v any) string {
	b, _ := jsonMarshal(v)
	return string(b)
}

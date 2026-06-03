package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/security"
)

type AuthHandler struct {
	JWT *auth.JWT
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{JWT: auth.NewJWTFromEnv()}
}

type loginReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	InviteCode  string `json:"invite_code"`
	DisplayName string `json:"display_name"`
	Action      string `json:"action"` // "login" / "register"
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名与密码必填"})
	}
	// 账号锁定检查
	cfg := auth.LoadLockoutCfg()
	if locked, left := auth.IsLocked(req.Username, cfg); locked {
		security.Record(security.AuditEvent{Action: "login_blocked", IP: c.IP(), Detail: "username=" + req.Username})
		return c.Status(429).JSON(fiber.Map{
			"error":          "账户已锁定",
			"retry_after_sec": int(left.Seconds()),
		})
	}
	u, err := auth.VerifyLogin(req.Username, req.Password)
	if err != nil {
		_, lerr := auth.CheckAndLock(req.Username, c.IP(), cfg)
		security.Record(security.AuditEvent{Action: "login_fail", IP: c.IP(), Detail: "username=" + req.Username})
		if lerr != nil {
			return c.Status(429).JSON(fiber.Map{"error": lerr.Error()})
		}
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	_ = auth.ClearLockState(req.Username)
	tok, _ := h.JWT.Sign(auth.Claims{
		UserID: u.ID, Username: u.Username, Admin: u.IsAdmin, SuperAdm: u.IsSuperAdmin, TokenType: "access",
	})
	auth.RecordLogin(u.ID, c.IP())
	security.Record(security.AuditEvent{ActorID: u.ID, Action: "login_ok", IP: c.IP()})
	return c.JSON(fiber.Map{"token": tok, "user": u})
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.InviteCode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "注册需要邀请码"})
	}
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	if _, _, err := auth.FindUserByName(req.Username); err == nil {
		return c.Status(409).JSON(fiber.Map{"error": "用户已存在"})
	}
	// 密码强度
	if err := checkPasswordStrength(req.Password); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	u, err := auth.CreateUser(req.Username, req.Password, false, false)
	if err != nil {
		security.Record(security.AuditEvent{Action: "register_fail", IP: c.IP(), Detail: "username=" + req.Username + " err=" + err.Error()})
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if err := auth.ConsumeInvite(req.InviteCode, u.ID); err != nil {
		_ = auth.BanUser(u.ID, true)
		security.Record(security.AuditEvent{ActorID: u.ID, Action: "register_invite_fail", IP: c.IP(), Detail: err.Error()})
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	tok, _ := h.JWT.Sign(auth.Claims{
		UserID: u.ID, Username: u.Username, Admin: u.IsAdmin, SuperAdm: u.IsSuperAdmin, TokenType: "access",
	})
	auth.RecordLogin(u.ID, c.IP())
	security.Record(security.AuditEvent{ActorID: u.ID, Action: "register_ok", IP: c.IP()})
	return c.JSON(fiber.Map{"token": tok, "user": u})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	u, err := auth.GetUser(uid)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "用户不存在"})
	}
	return c.JSON(u)
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	uname, _ := c.Locals(CtxUsername).(string)
	var req changePwdReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if err := checkPasswordStrength(req.NewPassword); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if err := auth.UpdatePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		security.Record(security.AuditEvent{ActorID: uid, Action: "password_change_fail", IP: c.IP(), Detail: err.Error()})
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	security.Record(security.AuditEvent{ActorID: uid, Action: "password_change_ok", IP: c.IP(), Detail: "u=" + uname})
	return c.JSON(fiber.Map{"ok": true})
}

type localeReq struct {
	Locale string `json:"locale"`
}

func (h *AuthHandler) SetLocale(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	var req localeReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.Locale != "zh-CN" && req.Locale != "en-US" {
		return c.Status(400).JSON(fiber.Map{"error": "不支持的语言"})
	}
	_ = auth.SetLocale(uid, req.Locale)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "logout", IP: c.IP()})
	return c.JSON(fiber.Map{"ok": true})
}

// checkPasswordStrength 最低强度：8 位，含字母+数字
func checkPasswordStrength(pw string) error {
	if len(pw) < 8 {
		return errPasswordTooShort
	}
	hasLetter, hasDigit := false, false
	for _, r := range pw {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errPasswordWeak
	}
	return nil
}

var (
	errPasswordTooShort = fiber.NewError(400, "密码至少 8 位")
	errPasswordWeak     = fiber.NewError(400, "密码必须同时包含字母和数字")
)

var _ = config.C

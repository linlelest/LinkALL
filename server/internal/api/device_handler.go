package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/db"
	"github.com/linkall/server/internal/models"
	"github.com/linkall/server/internal/security"
)

type DeviceHandler struct {
	JWT *auth.JWT
}

func NewDeviceHandler() *DeviceHandler {
	return &DeviceHandler{JWT: auth.NewJWTFromEnv()}
}

type registerDeviceReq struct {
	DeviceCode        string `json:"device_code"`
	DevicePassword    string `json:"device_password"`
	Name              string `json:"name"`
	Platform          string `json:"platform"`
	OSVersion         string `json:"os_version"`
	AppVersion        string `json:"app_version"`
	AllowAnonymous    *bool  `json:"allow_anonymous"`
	RequireDeviceCode *bool  `json:"require_device_code"`
	AcceptConnections *bool  `json:"accept_connections"`
	Tag               string `json:"tag"`
	Notes             string `json:"notes"`
}

func (h *DeviceHandler) Register(c *fiber.Ctx) error {
	var req registerDeviceReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.DeviceCode == "" || req.DevicePassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": "device_code 与 device_password 必填"})
	}
	if len(req.DevicePassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "设备码至少 6 位"})
	}
	if _, _, err := models.FindDeviceByCode(req.DeviceCode); err == nil {
		return c.Status(409).JSON(fiber.Map{"error": "设备编号已存在"})
	}
	d := &models.Device{
		Name:              req.Name,
		Platform:          req.Platform,
		OSVersion:         req.OSVersion,
		AppVersion:        req.AppVersion,
		AllowAnonymous:    config.C.AllowAnonymousDef,
		RequireDeviceCode: config.C.RequireDeviceCodeDef,
		AcceptConnections: true,
		Tag:               req.Tag,
		Notes:             req.Notes,
	}
	if req.AllowAnonymous != nil {
		d.AllowAnonymous = *req.AllowAnonymous
	}
	if req.RequireDeviceCode != nil {
		d.RequireDeviceCode = *req.RequireDeviceCode
	}
	if req.AcceptConnections != nil {
		d.AcceptConnections = *req.AcceptConnections
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	if uid > 0 {
		d.OwnerID = &uid
	}
	if err := models.RegisterDevice(d, req.DeviceCode, req.DevicePassword); err != nil {
		security.Record(security.AuditEvent{ActorID: uid, Action: "device_register_fail", IP: c.IP(), Target: req.DeviceCode, Detail: err.Error()})
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	tok, _ := h.JWT.Sign(auth.Claims{
		UserID:    d.ID,
		Username:  d.DeviceCode,
		TokenType: "device",
		DeviceID:  d.ID,
	})
	security.Record(security.AuditEvent{ActorID: uid, Action: "device_register_ok", IP: c.IP(), Target: d.DeviceCode})
	return c.JSON(fiber.Map{"device": d, "token": tok})
}

func (h *DeviceHandler) Login(c *fiber.Ctx) error {
	var req struct {
		DeviceCode     string `json:"device_code"`
		DevicePassword string `json:"device_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	d, ph, err := models.FindDeviceByCode(req.DeviceCode)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "设备编号不存在"})
	}
	ok, _ := auth.VerifyPassword(req.DevicePassword, ph, auth.DefaultParams())
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "设备码错误"})
	}
	_ = models.UpdateDeviceMeta(d.DeviceCode, "", "", "", c.IP(), "", "")
	tok, _ := h.JWT.Sign(auth.Claims{
		UserID:    d.ID,
		Username:  d.DeviceCode,
		TokenType: "device",
		DeviceID:  d.ID,
	})
	return c.JSON(fiber.Map{"device": d, "token": tok})
}

func (h *DeviceHandler) List(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	adm, _ := c.Locals(CtxIsAdmin).(bool)
	if adm {
		ds, err := models.ListAllDevices()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(ds)
	}
	ds, err := models.ListDevicesByOwner(uid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ds)
}

func (h *DeviceHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	d, err := models.FindDeviceByID(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "设备不存在"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	adm, _ := c.Locals(CtxIsAdmin).(bool)
	if !adm && d.OwnerID != nil && *d.OwnerID != uid {
		return c.Status(403).JSON(fiber.Map{"error": "无权限"})
	}
	return c.JSON(d)
}

type updateDeviceReq struct {
	Name              *string `json:"name"`
	AllowAnonymous    *bool   `json:"allow_anonymous"`
	RequireDeviceCode *bool   `json:"require_device_code"`
	AcceptConnections *bool   `json:"accept_connections"`
	Tag               *string `json:"tag"`
	Notes             *string `json:"notes"`
}

func (h *DeviceHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	d, err := models.FindDeviceByID(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "设备不存在"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	adm, _ := c.Locals(CtxIsAdmin).(bool)
	if !adm && d.OwnerID != nil && *d.OwnerID != uid {
		return c.Status(403).JSON(fiber.Map{"error": "无权限"})
	}
	var req updateDeviceReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if err := models.UpdateDeviceFlags(d.ID, req.AllowAnonymous, req.RequireDeviceCode, req.AcceptConnections); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	// 名称/标签/备注
	if req.Name != nil {
		_ = models.UpdateDeviceMeta(d.DeviceCode, *req.Name, "", "", "", "", "")
	}
	if req.Tag != nil {
		_ = models.UpdateDeviceMeta(d.DeviceCode, "", "", "", "", *req.Tag, "")
	}
	if req.Notes != nil {
		_ = models.UpdateDeviceMeta(d.DeviceCode, "", "", "", "", "", *req.Notes)
	}
	out, _ := models.FindDeviceByID(d.ID)
	return c.JSON(out)
}

type resetDeviceReq struct {
	NewCode     string `json:"new_code"`
	NewPassword string `json:"new_password"`
}

func (h *DeviceHandler) ResetCode(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	d, err := models.FindDeviceByID(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "设备不存在"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	adm, _ := c.Locals(CtxIsAdmin).(bool)
	if !adm && d.OwnerID != nil && *d.OwnerID != uid {
		return c.Status(403).JSON(fiber.Map{"error": "无权限"})
	}
	var req resetDeviceReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.NewCode == "" {
		req.NewCode = mustGenCode()
	}
	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "设备码至少 6 位"})
	}
	if err := models.ResetDeviceCode(d.ID, req.NewCode, req.NewPassword); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out, _ := models.FindDeviceByID(d.ID)
	return c.JSON(out)
}

func (h *DeviceHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	d, err := models.FindDeviceByID(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "设备不存在"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	adm, _ := c.Locals(CtxIsAdmin).(bool)
	sa, _ := c.Locals(CtxIsSuper).(bool)
	if !adm && d.OwnerID != nil && *d.OwnerID != uid {
		return c.Status(403).JSON(fiber.Map{"error": "无权限"})
	}
	if d.OwnerID == nil && !sa {
		return c.Status(403).JSON(fiber.Map{"error": "无主设备仅超级管理员可删除"})
	}
	if _, err := db.DB.Exec(`DELETE FROM devices WHERE id=?`, d.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	security.Record(security.AuditEvent{ActorID: uid, Action: "device_delete", IP: c.IP(), Target: d.DeviceCode})
	return c.JSON(fiber.Map{"ok": true})
}

func mustGenCode() string {
	c, _ := auth.GenerateDeviceCode()
	return c
}

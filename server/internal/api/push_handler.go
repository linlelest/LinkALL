// FCM token 注册 + WoL + 推送触发 handlers
package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/push"
)

// === Client: FCM token 上报 ===

type registerTokenReq struct {
	DeviceCode string `json:"device_code"`
	Token      string `json:"token"`
	AppVersion string `json:"app_version"`
}

func registerFCMToken(d *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req registerTokenReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.DeviceCode == "" || req.Token == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing field"})
		}
		if err := push.RegisterToken(d, req.DeviceCode, req.Token, req.AppVersion); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// === Admin: 唤醒设备 ===

type wakeReq struct {
	DeviceID string `json:"device_id"`
	Method   string `json:"method"` // "wol" | "fcm" | "both" | default: both
}

func adminWakeDevice(d *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req wakeReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.DeviceID == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "device_id required"})
		}
		// 查设备信息
		var deviceCode, lastIP, lastMAC string
		err := d.QueryRow(`
			SELECT d.device_code,
			       (SELECT ip FROM net_leases WHERE device_id=d.id ORDER BY leased_at DESC LIMIT 1),
			       (SELECT mac FROM net_leases WHERE device_id=d.id ORDER BY leased_at DESC LIMIT 1)
			FROM devices d WHERE d.id=? LIMIT 1
		`, req.DeviceID).Scan(&deviceCode, &lastIP, &lastMAC)
		if err == sql.ErrNoRows {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "device not found"})
		}
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		results := fiber.Map{
			"device_id":    req.DeviceID,
			"device_code":  deviceCode,
			"wol_sent":     false,
			"fcm_sent":     false,
			"online_already": false,
		}
		// 1) 检查是否在线
		var online int
		_ = d.QueryRow(`SELECT COUNT(*) FROM device_sessions WHERE device_id=? AND last_seen > unixepoch() - 60`, req.DeviceID).Scan(&online)
		if online > 0 {
			results["online_already"] = true
			return c.JSON(results)
		}
		method := req.Method
		if method == "" {
			method = "both"
		}
		// 2) WoL
		if method == "wol" || method == "both" {
			if lastMAC != "" {
				bcast := "255.255.255.255"
				if lastIP != "" {
					if b, err := push.ResolveBroadcastAddr(lastIP, "255.255.255.0"); err == nil {
						bcast = b
					}
				}
				if err := push.SendMagicPacket(bcast, lastMAC); err == nil {
					results["wol_sent"] = true
					results["wol_target"] = bcast
				} else {
					results["wol_error"] = err.Error()
				}
			}
		}
		// 3) FCM
		if method == "fcm" || method == "both" {
			cfg := config.C
			if cfg.FCMProjectID != "" && cfg.FCMOAuthToken != "" {
				tok, err := push.GetToken(d, deviceCode)
				if err == nil && tok != "" {
					fc := push.NewFCMClient(cfg.FCMProjectID, cfg.FCMOAuthToken)
					if err := fc.Send(c.Context(), tok, "LinkALL", "远程唤醒请求", map[string]string{
						"device_id":   req.DeviceID,
						"device_code": deviceCode,
					}); err == nil {
						results["fcm_sent"] = true
					} else {
						results["fcm_error"] = err.Error()
					}
				}
			}
		}
		return c.JSON(results)
	}
}

// === Admin: 列出 FCM tokens（调试用） ===

func adminListFCMTokens(d *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limit := intQuery(c, "limit", 100)
		rows, err := d.Query(`
			SELECT id, device_code, platform, app_version, last_seen, revoked
			FROM fcm_tokens ORDER BY last_seen DESC LIMIT ?
		`, limit)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()
		var out []fiber.Map
		for rows.Next() {
			var id, dev, plat, ver string
			var lastSeen int64
			var revoked int
			if err := rows.Scan(&id, &dev, &plat, &ver, &lastSeen, &revoked); err == nil {
				out = append(out, fiber.Map{
					"id":          id,
					"device_code": dev,
					"platform":    plat,
					"app_version": ver,
					"last_seen":   lastSeen,
					"revoked":     revoked != 0,
				})
			}
		}
		return c.JSON(fiber.Map{"tokens": out})
	}
}

func intQuery(c *fiber.Ctx, k string, def int) int {
	v := c.Query(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

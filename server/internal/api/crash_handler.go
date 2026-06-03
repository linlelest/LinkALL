// 客户端崩溃 / 日志上报：生产诊断的核心入口
package api

import (
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/db"
	"github.com/linkall/server/internal/security"
)

type crashReportReq struct {
	DeviceCode string `json:"device_code"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	OSVersion  string `json:"os_version"`
	Level      string `json:"level"` // "fatal" | "error" | "warn" | "info" | "debug"
	Source     string `json:"source"`
	Message    string `json:"message"`
	Stack      string `json:"stack"`
	Extra      string `json:"extra"`
}

// POST /api/crash
// 公开（按 IP 限流，IP 5/min）—— 客户端无 user/device token 时也能上报
func crashHandler(c *fiber.Ctx) error {
	var req crashReportReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{"error": "message 必填"})
	}
	if req.Level == "" {
		req.Level = "error"
	}
	if req.Platform == "" {
		req.Platform = "unknown"
	}
	// 截断避免大 payload
	if len(req.Message) > 4096 { req.Message = req.Message[:4096] }
	if len(req.Stack) > 16384 { req.Stack = req.Stack[:16384] }
	if len(req.Extra) > 4096 { req.Extra = req.Extra[:4096] }

	var deviceIDPtr *int64
	if req.DeviceCode != "" {
		if id, err := lookupDeviceIDByCode(req.DeviceCode); err == nil {
			deviceIDPtr = &id
		}
	}
	_ = deviceIDPtr // currently unused; reserved for future FK

	now := nowUnix()
	_, err := db.DB.Exec(`INSERT INTO crash_logs
		(actor_id, device_code, platform, app_version, os_version, level, source, message, stack, extra, client_ip, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		nil, req.DeviceCode, req.Platform, req.AppVersion, req.OSVersion,
		req.Level, req.Source, req.Message, req.Stack, req.Extra, c.IP(), now)
	if err != nil {
		log.Printf("[crash] insert: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "internal"})
	}
	if req.Level == "fatal" {
		log.Printf("[crash][FATAL] device=%s platform=%s msg=%s", req.DeviceCode, req.Platform, req.Message)
		// 写审计
		security.Record(security.AuditEvent{Action: "client_crash", IP: c.IP(), Detail: "device=" + req.DeviceCode + " msg=" + truncStr(req.Message, 200)})
	}
	return c.JSON(fiber.Map{"ok": true})
}

type logReportReq struct {
	DeviceCode string `json:"device_code"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	Level      string `json:"level"`
	Source     string `json:"source"`
	// 批量日志条目
	Entries []logEntry `json:"entries"`
}

type logEntry struct {
	Ts      int64  `json:"ts"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

// POST /api/log
// 批量日志上报（INFO 级别以上），用于远程看客户端运行状态
func logReportHandler(c *fiber.Ctx) error {
	var req logReportReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if len(req.Entries) == 0 {
		return c.JSON(fiber.Map{"ok": true, "stored": 0})
	}
	// 限制批量大小（防滥用）
	if len(req.Entries) > 500 {
		req.Entries = req.Entries[:500]
	}
	now := nowUnix()
	stored := 0
	for _, e := range req.Entries {
		if e.Message == "" { continue }
		if len(e.Message) > 4096 { e.Message = e.Message[:4096] }
		if e.Level == "" { e.Level = req.Level }
		if e.Level == "" { e.Level = "info" }
		if e.Source == "" { e.Source = req.Source }
		_, err := db.DB.Exec(`INSERT INTO crash_logs
			(actor_id, device_code, platform, app_version, os_version, level, source, message, stack, extra, client_ip, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			nil, req.DeviceCode, req.Platform, req.AppVersion, "",
			e.Level, e.Source, e.Message, "", "", c.IP(), now)
		if err == nil { stored++ }
	}
	return c.JSON(fiber.Map{"ok": true, "stored": stored})
}

// GET /api/admin/crash-logs?level=&device_code=&from=&to=&limit=
func adminListCrashLogs(c *fiber.Ctx) error {
	level := c.Query("level")
	deviceCode := c.Query("device_code")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	limitStr := c.Query("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 { limit = 100 }
	q := `SELECT id, COALESCE(actor_id, 0), COALESCE(device_code, ''), platform, COALESCE(app_version, ''),
		COALESCE(os_version, ''), level, COALESCE(source, ''), message, COALESCE(stack, ''),
		COALESCE(extra, ''), COALESCE(client_ip, ''), created_at
		FROM crash_logs WHERE 1=1`
	args := []any{}
	if level != "" {
		q += ` AND level=?`
		args = append(args, level)
	}
	if deviceCode != "" {
		q += ` AND device_code=?`
		args = append(args, deviceCode)
	}
	if fromStr != "" {
		if from, err := strconv.ParseInt(fromStr, 10, 64); err == nil {
			q += ` AND created_at>=?`
			args = append(args, from)
		}
	}
	if toStr != "" {
		if to, err := strconv.ParseInt(toStr, 10, 64); err == nil {
			q += ` AND created_at<=?`
			args = append(args, to)
		}
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.DB.Query(q, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	type Row struct {
		ID         int64  `json:"id"`
		ActorID    int64  `json:"actor_id"`
		DeviceCode string `json:"device_code"`
		Platform   string `json:"platform"`
		AppVersion string `json:"app_version"`
		OSVersion  string `json:"os_version"`
		Level      string `json:"level"`
		Source     string `json:"source"`
		Message    string `json:"message"`
		Stack      string `json:"stack"`
		Extra      string `json:"extra"`
		ClientIP   string `json:"client_ip"`
		CreatedAt  int64  `json:"created_at"`
	}
	out := []Row{}
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.ActorID, &r.DeviceCode, &r.Platform, &r.AppVersion, &r.OSVersion,
			&r.Level, &r.Source, &r.Message, &r.Stack, &r.Extra, &r.ClientIP, &r.CreatedAt); err != nil {
			continue
		}
		out = append(out, r)
	}
	return c.JSON(fiber.Map{"logs": out, "count": len(out)})
}

// GET /api/admin/crash-logs/stats
func adminCrashStats(c *fiber.Ctx) error {
	rows, err := db.DB.Query(`SELECT level, COUNT(*) FROM crash_logs WHERE created_at>? GROUP BY level`, nowUnix()-86400)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	stats := fiber.Map{}
	for rows.Next() {
		var lvl string
		var n int64
		_ = rows.Scan(&lvl, &n)
		stats[lvl] = n
	}
	var total int64
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM crash_logs`).Scan(&total)
	return c.JSON(fiber.Map{"by_level_24h": stats, "total": total})
}

func lookupDeviceIDByCode(code string) (int64, error) {
	var id int64
	err := db.DB.QueryRow(`SELECT id FROM devices WHERE device_code=?`, code).Scan(&id)
	return id, err
}

func truncStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n { return s[:n] }
	return s
}

package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/models"
	"github.com/linkall/server/internal/ota"
	"github.com/linkall/server/internal/security"
)

type OTAHandler struct{}

func NewOTAHandler() *OTAHandler { return &OTAHandler{} }

func (h *OTAHandler) List(c *fiber.Ctx) error {
	include := c.QueryBool("include_revoked", false)
	ps, err := models.ListOTA(include)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ps)
}

func (h *OTAHandler) Upload(c *fiber.Ctx) error {
	platform := strings.ToLower(strings.TrimSpace(c.FormValue("platform")))
	version := strings.TrimSpace(c.FormValue("version"))
	channel := strings.ToLower(strings.TrimSpace(c.FormValue("channel")))
	notes := c.FormValue("release_notes")
	force := c.FormValue("force_update") == "true" || c.FormValue("force_update") == "1"
	minV := c.FormValue("min_supported_version")
	providedSig := c.FormValue("signature") // 客户端可自带
	if platform == "" || version == "" {
		return c.Status(400).JSON(fiber.Map{"error": "platform 和 version 必填"})
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "未找到上传文件"})
	}
	if channel == "" {
		channel = "stable"
	}
	dir := filepath.Join(config.C.OTADir, platform)
	_ = os.MkdirAll(dir, 0o755)
	safeName := sanitizeFilename(fh.Filename)
	finalName := fmt.Sprintf("%s-%s-%s%s", platform, version, channel, filepath.Ext(safeName))
	finalPath := filepath.Join(dir, finalName)
	src, err := fh.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer src.Close()
	dst, err := os.Create(finalPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer dst.Close()
	hasher := sha256.New()
	mw := io.MultiWriter(dst, hasher)
	n, err := io.Copy(mw, src)
	if err != nil {
		_ = os.Remove(finalPath)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	sum := hasher.Sum(nil)
	sha256hex := hex.EncodeToString(sum)
	sig := providedSig
	if sig == "" {
		// 自动用服务端私钥签名
		_, _, _ = ota.S.SignFile(nil) // 仅占位，保持函数签名一致
		sig = base64.StdEncoding.EncodeToString(otaSignSum(sum))
	}
	p := &models.OTAPackage{
		Platform:            platform,
		Version:             version,
		Channel:             channel,
		FileName:            safeName,
		FilePath:            finalPath,
		FileSize:            n,
		SHA256:              sha256hex,
		Signature:           sig,
		ReleaseNotes:        notes,
		ForceUpdate:         force,
		MinSupportedVersion: minV,
	}
	if err := models.CreateOTA(p); err != nil {
		_ = os.Remove(finalPath)
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "ota_upload", IP: c.IP(), Target: fmt.Sprintf("%s/%s/%s", platform, version, channel), Detail: fmt.Sprintf("size=%d sha=%s", n, sha256hex)})
	return c.JSON(p)
}

func (h *OTAHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	var p models.OTAPackage
	if err := c.BodyParser(&p); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	p.ID = int64(id)
	if err := models.UpdateOTA(&p); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *OTAHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	if err := models.DeleteOTA(int64(id)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	security.Record(security.AuditEvent{ActorID: uid, Action: "ota_delete", IP: c.IP(), Target: c.Params("id")})
	return c.JSON(fiber.Map{"ok": true})
}

type otaCheckReq struct {
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

func (h *OTAHandler) Check(c *fiber.Ctx) error {
	platform := strings.ToLower(strings.TrimSpace(c.Query("platform")))
	current := strings.TrimSpace(c.Query("version"))
	if platform == "" {
		var r otaCheckReq
		_ = c.BodyParser(&r)
		if r.Platform != "" {
			platform = r.Platform
		}
		if r.Version != "" {
			current = r.Version
		}
	}
	if platform == "" {
		return c.Status(400).JSON(fiber.Map{"error": "platform 必填"})
	}
	p, err := models.GetLatestOTA(platform, current)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(fiber.Map{
			"has_update": false,
			"ota_pubkey": ota.S.PublicKeyB64(),
			"ota_keyid":  ota.S.KeyID(),
		})
	}
	has := versionGreater(p.Version, current)
	return c.JSON(fiber.Map{
		"has_update":            has,
		"force_update":          p.ForceUpdate,
		"platform":              p.Platform,
		"version":               p.Version,
		"channel":               p.Channel,
		"release_notes":         p.ReleaseNotes,
		"min_supported_version": p.MinSupportedVersion,
		"file_size":             p.FileSize,
		"sha256":                p.SHA256,
		"signature":             p.Signature,
		"download_url":          fmt.Sprintf("/api/ota/download/%d", p.ID),
		"created_at":            p.CreatedAt,
		"ota_pubkey":            ota.S.PublicKeyB64(),
		"ota_keyid":             ota.S.KeyID(),
	})
}

func (h *OTAHandler) Download(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	p, path, err := models.GetOTAPackage(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "package not found"})
	}
	if p.Revoked {
		return c.Status(410).JSON(fiber.Map{"error": "package revoked"})
	}
	models.IncrementOTADownloads(p.ID)
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", `attachment; filename="`+p.FileName+`"`)
	c.Set("X-Checksum-SHA256", p.SHA256)
	c.Set("X-Package-Version", p.Version)
	c.Set("X-Package-Platform", p.Platform)
	c.Set("X-Signature", p.Signature)
	c.Set("X-OTA-Pubkey", ota.S.PublicKeyB64())
	return c.SendFile(path, false)
}

// ===== 邀请码 =====

type createInviteReq struct {
	MaxUses  int    `json:"max_uses"`
	TTLHours int    `json:"ttl_hours"`
	Note     string `json:"note"`
}

func (h *OTAHandler) ListInvites(c *fiber.Ctx) error {
	us, err := auth.ListInvites()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(us)
}

func (h *OTAHandler) CreateInvite(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	var req createInviteReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	if req.TTLHours <= 0 {
		req.TTLHours = config.C.InviteDefaultTTLHrs
	}
	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}
	inv, err := auth.CreateInvite(uid, req.MaxUses, req.TTLHours, req.Note)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	security.Record(security.AuditEvent{ActorID: uid, Action: "invite_create", IP: c.IP(), Target: inv.Code})
	return c.JSON(inv)
}

func (h *OTAHandler) RevokeInvite(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	if err := auth.RevokeInvite(int64(id), uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	security.Record(security.AuditEvent{ActorID: uid, Action: "invite_revoke", IP: c.IP(), Target: c.Params("id")})
	return c.JSON(fiber.Map{"ok": true})
}

// ===== helpers =====

func sanitizeFilename(s string) string {
	s = filepath.Base(s)
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	if s == "" || s == "." {
		s = "package-" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	return s
}

func versionGreater(latest, current string) bool {
	parse := func(s string) []int {
		s = strings.TrimPrefix(strings.TrimSpace(s), "v")
		parts := strings.Split(s, ".")
		out := []int{}
		for _, p := range parts {
			n := 0
			for _, ch := range p {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					break
				}
			}
			out = append(out, n)
		}
		return out
	}
	a := parse(latest)
	b := parse(current)
	for i := 0; i < len(a) || i < len(b); i++ {
		ai, bi := 0, 0
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai > bi {
			return true
		}
		if ai < bi {
			return false
		}
	}
	return false
}

// 客户端也可用此函数验签：base64(ed25519_sign(sha256_bytes))
func otaSignSum(sum []byte) []byte {
	// 取自 ota.Signer
	if ota.S == nil {
		return nil
	}
	// 延迟导入 ed25519，避免循环依赖
	return otaSignSumImpl(sum)
}

// 包装：避免与 ota 包内方法重名导致递归
func otaSignSumImpl(sum []byte) []byte {
	return ota.SignBytes(sum)
}

var _ = errors.New

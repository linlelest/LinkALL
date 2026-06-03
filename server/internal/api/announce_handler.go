package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/models"
)

type AnnounceHandler struct{}

func NewAnnounceHandler() *AnnounceHandler { return &AnnounceHandler{} }

func (h *AnnounceHandler) List(c *fiber.Ctx) error {
	include := c.QueryBool("include_revoked", false)
	as, err := models.ListAnnouncements(include)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(as)
}

func (h *AnnounceHandler) Unread(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	as, err := models.ListUnreadAnnouncements(uid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(as)
}

func (h *AnnounceHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	a, err := models.GetAnnouncement(int64(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(a)
}

type announceReq struct {
	Title      string `json:"title"`
	ContentMD  string `json:"content_md"`
	Platform   string `json:"platform"`
	MinVersion string `json:"min_version"`
	Pinned     bool   `json:"pinned"`
	ForceRead  bool   `json:"force_read"`
	Signature  string `json:"signature"`
}

func (h *AnnounceHandler) Create(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	var req announceReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	a := &models.Announcement{
		AuthorID:   uid,
		Title:      req.Title,
		ContentMD:  req.ContentMD,
		Platform:   req.Platform,
		MinVersion: req.MinVersion,
		Pinned:     req.Pinned,
		ForceRead:  req.ForceRead,
		Signature:  req.Signature,
	}
	if err := models.CreateAnnouncement(a); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(a)
}

func (h *AnnounceHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	uid, _ := c.Locals(CtxUserID).(int64)
	var req announceReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式错误"})
	}
	a := &models.Announcement{
		ID:         int64(id),
		AuthorID:   uid,
		Title:      req.Title,
		ContentMD:  req.ContentMD,
		Platform:   req.Platform,
		MinVersion: req.MinVersion,
		Pinned:     req.Pinned,
		ForceRead:  req.ForceRead,
		Signature:  req.Signature,
	}
	if err := models.UpdateAnnouncement(a); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(a)
}

func (h *AnnounceHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	if err := models.DeleteAnnouncement(int64(id)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AnnounceHandler) MarkRead(c *fiber.Ctx) error {
	uid, _ := c.Locals(CtxUserID).(int64)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "id 无效"})
	}
	if err := models.MarkAnnouncementRead(int64(id), uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

var _ = strconv.Itoa // 防止未使用

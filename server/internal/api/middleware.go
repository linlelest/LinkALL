package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/security"
)

const (
	CtxUserID   = "uid"
	CtxIsAdmin  = "adm"
	CtxIsSuper  = "sa"
	CtxUsername = "uname"
)

func JWTAuth(j *auth.JWT) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if h == "" {
			h = c.Cookies("token")
			if h != "" {
				h = "Bearer " + h
			}
		}
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(401).JSON(fiber.Map{"error": "missing token"})
		}
		tok := strings.TrimPrefix(h, "Bearer ")
		claims, err := j.Parse(tok)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid token"})
		}
		if claims.TokenType != "access" {
			return c.Status(401).JSON(fiber.Map{"error": "wrong token type"})
		}
		c.Locals(CtxUserID, claims.UserID)
		c.Locals(CtxIsAdmin, claims.Admin)
		c.Locals(CtxIsSuper, claims.SuperAdm)
		c.Locals(CtxUsername, claims.Username)
		return c.Next()
	}
}

func DeviceAuth(j *auth.JWT) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(401).JSON(fiber.Map{"error": "missing device token"})
		}
		tok := strings.TrimPrefix(h, "Bearer ")
		claims, err := j.Parse(tok)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid device token"})
		}
		if claims.TokenType != "device" {
			return c.Status(401).JSON(fiber.Map{"error": "not a device token"})
		}
		c.Locals("device_id", claims.DeviceID)
		c.Locals("device_code", claims.Username)
		return c.Next()
	}
}

func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		adm, _ := c.Locals(CtxIsAdmin).(bool)
		sa, _ := c.Locals(CtxIsSuper).(bool)
		if !(adm || sa) {
			return c.Status(403).JSON(fiber.Map{"error": "需要管理员权限"})
		}
		return c.Next()
	}
}

func SuperAdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sa, _ := c.Locals(CtxIsSuper).(bool)
		if !sa {
			return c.Status(403).JSON(fiber.Map{"error": "需要超级管理员权限"})
		}
		return c.Next()
	}
}

// RateLimit 限流中间件（按 scope+identity）
func RateLimit(scope string, identity func(*fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := identity(c)
		if id == "" {
			id = c.IP()
		}
		lim := security.GetLimiter()
		ok, _ := lim.Allow(scope, id)
		if !ok {
			return c.Status(429).JSON(fiber.Map{"error": "请求过于频繁，请稍后再试"})
		}
		return c.Next()
	}
}

func IpRateLimit(scope string) fiber.Handler {
	return RateLimit(scope, func(c *fiber.Ctx) string { return c.IP() })
}

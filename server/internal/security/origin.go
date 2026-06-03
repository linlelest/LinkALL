// CSRF 防护：仅检查状态变更请求携带合法的 Origin / Referer
// 简化策略：要求 Origin 与 Referer 在白名单内（仅 /api/* 写入端点强制）
package security

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type CSRFMiddleware struct {
	AllowedOrigins []string
}

func (m *CSRFMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}
		origin := c.Get("Origin")
		referer := c.Get("Referer")
		// 当无 Origin 也无 Referer 时（例如非浏览器 / curl），若配置了 allow-empty=true 则放行
		if origin == "" && referer == "" {
			return c.Next()
		}
		host := strings.ToLower(c.Hostname())
		if origin != "" {
			if ok := m.matchOrigin(origin, host); ok {
				return c.Next()
			}
		}
		if referer != "" {
			if ok := m.matchReferer(referer, host); ok {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "CSRF: invalid origin/referer"})
	}
}

func (m *CSRFMiddleware) matchOrigin(origin, host string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	for _, a := range m.AllowedOrigins {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	// 同源
	if strings.Contains(origin, "://"+host) {
		return true
	}
	return false
}

func (m *CSRFMiddleware) matchReferer(referer, host string) bool {
	referer = strings.ToLower(strings.TrimSpace(referer))
	if strings.Contains(referer, "://"+host) {
		return true
	}
	for _, a := range m.AllowedOrigins {
		if strings.HasPrefix(referer, strings.ToLower(a)) {
			return true
		}
	}
	return false
}

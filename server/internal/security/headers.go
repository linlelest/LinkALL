// 安全工具：CSP / HSTS / X-Content-Type-Options / Referrer-Policy 等响应头
package security

import "github.com/gofiber/fiber/v2"

func SecurityHeaders(cfg HeaderConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 严格 CSP（限制内联脚本/样式为安全范围）
		// 注意：Tauri / Flutter webview 不需要 CSP；这里配置的是 Web 前端
		// Vite+Svelte 5 生产构建不需要 unsafe-eval（编译期代码生成）
		csp := "default-src 'self'; " +
			"script-src 'self' blob:; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: blob:; " +
			"font-src 'self' data:; " +
			"connect-src 'self' ws: wss:; " +
			"media-src 'self' blob:; " +
			"object-src 'none'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		c.Set("Content-Security-Policy", csp)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		if cfg.HSTS {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// 隐藏 Server 头
		c.Set("Server", "LinkALL")
		return c.Next()
	}
}

type HeaderConfig struct {
	HSTS bool
}

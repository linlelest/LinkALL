# LinkALL Admin Console

Svelte 5 + Vite 6 + Tailwind 4 单页管理控制台。

## 跑起来

```bash
cd admin
pnpm install
pnpm run dev          # http://localhost:5180，代理 /api → :8080
pnpm run build        # 输出 dist/
pnpm run check        # svelte-check 类型检查
```

> Vite 代理默认假设 LinkALL server 跑在 `http://127.0.0.1:8080`，
> 修改 `vite.config.ts` 的 `proxy['/api']` 改成你的服务端地址。

## 功能页面

| 页面 | 路由（hash 内部 state） | 说明 |
|---|---|---|
| 概览 | `dashboard` | 用户/设备/会话/崩溃计数 + 24h 崩溃分布 |
| 崩溃日志 | `crash` | 客户端上报的 panic / error / 日志；支持 level / device_code 过滤、stack 展开 |
| 设备 | `devices` | 设备列表 + 唤醒（WoL / FCM / both）+ 删除 |
| FCM 令牌 | `fcm` | Android controlled 上报的有效 token 列表 |
| 限流 | `rate` | 3 档预设切换（loose / medium / strict） |
| 审计 | `audit` | 服务端写出的 admin 操作日志 |

## 鉴权

- 登录用 `/api/auth/login` 拿 JWT，存 `localStorage.linkall.admin.jwt`
- 所有 `/api/admin/*` 自动带 `Authorization: Bearer <jwt>`
- 退出清 localStorage + reload

## 构建产物

`pnpm run build` 产物在 `admin/dist/`，可以挂到任何静态服务器（Nginx、Caddy、Cloudflare Pages）。
也可以让 LinkALL server 的 `/admin/*` 路径 serve 这份静态产物（见 `server/internal/api/router.go` 留的占位点）。

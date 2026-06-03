# LinkALL Web API 客户端参考

> 本文件描述 Web 前端与 Go 服务端之间的全部 HTTP/WS 接口契约。所有路由在 Go 服务端都已实现（见 `server/api.md`），本文件是 Web 前端的调用视角。

## 基础

- Base URL：默认当前浏览器 origin；登录页可填 `http://host:port` 覆盖并存储到 `localStorage.linkall.server`。
- Token：JWT，存到 `localStorage.linkall.token`，请求头 `Authorization: Bearer <token>`。
- 401：自动清空 token 并跳转到 `/login`。

## 错误格式

```json
{ "error": "用户已存在" }
```

## 端点清单

### Auth

| Method | Path | Body | 说明 |
|--------|------|------|------|
| POST | /api/auth/login | `{action,username,password,invite_code?}` | 登录或注册，登录后返回 `{token, user}` |
| GET | /api/auth/me | – | 当前用户 |
| POST | /api/auth/password | `{old_password,new_password}` | 改密 |
| POST | /api/auth/locale | `{locale}` | 切换语言 |
| POST | /api/auth/logout | – | 登出（前端清 token） |

### Devices

| Method | Path | Body | 说明 |
|--------|------|------|------|
| POST | /api/devices/register | `{device_code,device_password,name?,platform?,os_version?,app_version?,allow_anonymous?,require_device_code?,accept_connections?,tag?,notes?}` | 设备首次注册 |
| POST | /api/devices/login | `{device_code,device_password}` | 设备登录 |
| GET | /api/devices | – | 设备列表（管理员返回全部） |
| GET | /api/devices/:id | – | 设备详情 |
| PATCH | /api/devices/:id | `{name?,allow_anonymous?,require_device_code?,accept_connections?,tag?,notes?}` | 编辑 |
| POST | /api/devices/:id/reset-code | `{new_code?,new_password}` | 重置编号/设备码 |
| DELETE | /api/devices/:id | – | 删除 |

### Announcements

| Method | Path | Body | 说明 |
|--------|------|------|------|
| GET | /api/announcements | – | 列表（带 `?include_revoked=true`） |
| GET | /api/announcements/unread | – | 未读 |
| GET | /api/announcements/:id | – | 详情 |
| POST | /api/announcements/:id/read | – | 标记已读 |
| POST | /api/announcements | `{title,content_md,platform?,min_version?,pinned?,force_read?,signature?}` | 发布（管理员） |
| PATCH | /api/announcements/:id | 同上 | 编辑 |
| DELETE | /api/announcements/:id | – | 作废 |

### OTA

| Method | Path | Body | 说明 |
|--------|------|------|------|
| GET | /api/ota/list | – | 列表（`?include_revoked=true`） |
| GET | /api/ota/check | – | `?platform=...&version=...`，公开 |
| GET | /api/ota/download/:id | – | 下载文件流 |
| POST | /api/ota/upload | multipart/form-data | 字段：`file, platform, version, channel, release_notes, force_update, min_supported_version, signature` |
| PATCH | /api/ota/:id | `{...}` | 编辑 |
| DELETE | /api/ota/:id | – | 作废 |

### Invites

| Method | Path | Body | 说明 |
|--------|------|------|------|
| GET | /api/invites | – | 邀请码列表 |
| POST | /api/invites | `{max_uses,ttl_hours,note?}` | 生成 |
| DELETE | /api/invites/:id | – | 吊销 |

### Admin

| Method | Path | Body | 说明 |
|--------|------|------|------|
| GET | /api/admin/stats | – | 运行时统计 |
| GET | /api/admin/users | – | 用户列表 |
| PATCH | /api/admin/users/:id | `{banned?,is_admin?,is_super_admin?,new_password?}` | 编辑 |
| DELETE | /api/admin/users/:id | – | 删除（超级管理员） |

### Public

| Method | Path | Body | 说明 |
|--------|------|------|------|
| GET | /api/health | – | 健康检查 |
| GET | /api/config | – | ICE、官方地址、限额等 |

## 信令（WebSocket）

`GET /ws/signaling`（自动升级），文本帧，JSON 帧。详见 `src/lib/webrtc.ts` 与 `server/api.md` 的"信令"一节。Web 端 `ControlClient` 已封装好 `hello` / `request` / `offer` / `answer` / `ice` / `cmd` 等收发。

## 控制指令（DataChannel/信令 cmd）

`sendCmd` 可发送：
- `mouse` `{x,y,button,down,click_count,dx?,dy?,move?}`
- `key` `{code,key,down,mods:{ctrl,alt,shift,meta}}`
- `wheel` `{dx,dy}`
- `type` `{text}`
- `clip` `{text}`
- `file_send` `{transfer_id,name,size}`
- `file_data` `{transfer_id,offset,data(base64)}`
- `file_end` `{transfer_id,sha256}`
- `privacy` `{enabled}`
- `config` `{scale,bitrate_kbps,fps,codec}`

## 路由（前端）

基于 hash 的路由（`src/lib/router.ts`）：

| 路径 | 页面 | 角色 |
|------|------|------|
| `#/login` | Login | 匿名 |
| `#/dashboard` | Dashboard | 已登录 |
| `#/devices` | Devices | 已登录 |
| `#/connect` | Connect | 已登录 |
| `#/control/<device_code>` | Control | 已登录 |
| `#/announcements` | Announcements | 已登录 |
| `#/profile` | Profile | 已登录 |
| `#/admin/users` | Users | admin |
| `#/admin/invites` | Invites | admin |
| `#/admin/announcements` | Announcements | admin |
| `#/admin/ota` | OTA | admin |
| `#/admin/server` | Server | admin |

## 状态管理

`src/lib/auth.ts` 暴露 `token` 与 `user` 两个 Svelte store（持久化到 `localStorage`）。`src/i18n/index.ts` 暴露 `locale` 与 `t` derived store。

## 安全

- 不在 URL 中传 token，token 仅存在 `localStorage` / 请求头
- 切到 HTTPS 后建议加 `Secure; HttpOnly; SameSite=Strict` 的 cookie（待后端实现）
- 所有路由在 `App.svelte` 顶层做 `$token` 守卫

---

# LinkALL Web API (English)

This file documents the HTTP/WS contracts the web frontend uses against the Go server (which implements them; see `server/api.md`).

## Base
- Base URL defaults to the current origin. The sign-in page lets the user override it (stored in `localStorage.linkall.server`).
- JWT in `localStorage.linkall.token`, sent as `Authorization: Bearer <token>`.
- 401: token cleared, redirect to `#/login`.

## Error

```json
{ "error": "Chinese error message" }
```

## Endpoints

(Same list as the Chinese section above.)

## Signaling

`GET /ws/signaling` (text frames, JSON). The `ControlClient` in `src/lib/webrtc.ts` wraps `hello` / `request` / `offer` / `answer` / `ice` / `cmd`.

## Control commands

(Same list as the Chinese section above.)

## Routes (frontend hash router)

| Hash | Page | Auth |
|------|------|------|
| `#/login` | Login | guest |
| `#/dashboard` | Dashboard | auth |
| `#/devices` | Devices | auth |
| `#/connect` | Connect | auth |
| `#/control/<device_code>` | Control | auth |
| `#/announcements` | Announcements | auth |
| `#/profile` | Profile | auth |
| `#/admin/users` | Users | admin |
| `#/admin/invites` | Invites | admin |
| `#/admin/announcements` | Announcements | admin |
| `#/admin/ota` | OTA | admin |
| `#/admin/server` | Server | admin |

## State
`src/lib/auth.ts` exposes `token` and `user` Svelte stores (persisted to `localStorage`). `src/i18n/index.ts` exposes `locale` and the derived `t` store.

## Security
- Token only in `localStorage` / headers
- For HTTPS deployments, prefer `Secure; HttpOnly; SameSite=Strict` cookies (server-side support to be added)
- All routes guarded by `$token` in `App.svelte`

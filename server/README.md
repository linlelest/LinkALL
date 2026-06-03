# LinkALL Server

LinkALL 远控系统的服务端（Go 单二进制）。负责：用户/设备/邀请码管理、公告、OTA、信令转发（WebSocket + SDP/ICE）、REST API、JWT 鉴权、文件下载。

## 编译与运行

要求：Go **1.22+**，CGO 关闭（已使用纯 Go SQLite 驱动）。

```bash
cd server
cp .env.example .env
# 编辑 .env 至少改 JWT_SECRET
go build -trimpath -ldflags "-s -w" -o linkall-server ./cmd/server
./linkall-server
```

首次启动会在终端引导创建**超级管理员**（非交互式终端下会跳过，可手动用 CLI 子命令 `init-admin` 创建）。

## 端口与目录

- 默认监听 `:8080`，可通过 `HTTP_ADDR=:9000` 改。
- DB 默认 `./data/linkall.db`（SQLite + WAL）。
- OTA 包默认 `./data/ota/<platform>/...`。
- 静态前端 `./web/`（如已构建，会以 SPA 方式挂载在 `/`）。

## HTTPS

设置 `TLS_CERT` 与 `TLS_KEY` 自动启用 HTTPS。

## 关键 API 概览

详见 `server/api.md`。

- `POST /api/auth/login` — 登录/注册
- `GET  /api/auth/me` — 当前用户
- `POST /api/auth/password` — 改密
- `POST /api/auth/locale` — 切换语言

- `POST /api/devices/register` — 设备首次注册
- `POST /api/devices/login` — 设备登录
- `GET  /api/devices` — 设备列表
- `PATCH /api/devices/:id` — 改设备名/安全开关
- `POST /api/devices/:id/reset-code` — 重置设备编号/密码

- `GET  /api/announcements` — 公告列表
- `POST /api/announcements` — 发布（管理员）
- `POST /api/announcements/:id/read` — 标记已读

- `GET  /api/ota/check?platform=win64&version=1.0.0` — 拉取最新版本
- `GET  /api/ota/download/:id` — 下载包
- `POST /api/ota/upload` — 上传（管理员 multipart）

- `GET  /api/invites` — 邀请码列表
- `POST /api/invites` — 生成
- `DELETE /api/invites/:id` — 吊销

- `GET  /api/admin/stats` — 服务器统计
- `GET  /api/admin/users` — 用户管理
- `PATCH /api/admin/users/:id` — 改密/封禁/权限

- `GET  /ws/signaling` — WebRTC 信令

## WebRTC 信令协议

WebSocket 通道，文本帧，JSON。详见 `server/api.md` 的"信令章节"。

---

# LinkALL Server (English)

The Go backend for the LinkALL remote control system. A single static binary handling users/devices/invites, announcements, OTA, signaling relay, REST API, JWT auth, and file downloads.

## Build & Run

Requires: Go **1.22+**, CGO disabled (pure-Go SQLite driver).

```bash
cd server
cp .env.example .env
# Edit .env and change JWT_SECRET at minimum
go build -trimpath -ldflags "-s -w" -o linkall-server ./cmd/server
./linkall-server
```

On first run, the terminal will prompt to create a **super admin** account (skipped on non-interactive terminals; you can use the `init-admin` subcommand).

## Ports & Directories

- Listens on `:8080` by default; override with `HTTP_ADDR=:9000`.
- DB at `./data/linkall.db` (SQLite + WAL).
- OTA packages at `./data/ota/<platform>/...`.
- Web frontend at `./web/` (mounted as SPA on `/` if present).

## HTTPS

Set `TLS_CERT` and `TLS_KEY` to enable TLS.

## Highlights

See `server/api.md` for the full list.

- `POST /api/auth/login` — login or register
- `GET  /api/auth/me` — current user
- `POST /api/auth/password` — change password
- `POST /api/auth/locale` — switch locale

- `POST /api/devices/register` — first-time device register
- `POST /api/devices/login` — device login
- `GET  /api/devices` — list devices
- `PATCH /api/devices/:id` — change name / security flags
- `POST /api/devices/:id/reset-code` — reset code/password

- `GET  /api/announcements` — list announcements
- `POST /api/announcements` — publish (admin)
- `POST /api/announcements/:id/read` — mark as read

- `GET  /api/ota/check?platform=win64&version=1.0.0` — fetch latest
- `GET  /api/ota/download/:id` — download package
- `POST /api/ota/upload` — upload (admin multipart)

- `GET  /api/invites` — invite codes
- `POST /api/invites` — generate
- `DELETE /api/invites/:id` — revoke

- `GET  /api/admin/stats` — server stats
- `GET  /api/admin/users` — user list
- `PATCH /api/admin/users/:id` — change password/ban/role

- `GET  /ws/signaling` — WebRTC signaling

## WebRTC Signaling

WebSocket channel, text frames, JSON. See the "Signaling" section in `server/api.md`.

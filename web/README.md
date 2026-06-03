# LinkALL Web（控制端 + 管理后台）

> 基于 Svelte 5（Runes）+ Vite 6 + Tailwind 3 的单页应用。包含**控制端**（远程连接设备）与**管理后台**（用户/邀请/公告/OTA/服务器监控）。运行时与 Go 服务端配套（默认 `http://127.0.0.1:8080`）。

## 功能

- **登录/注册**：支持邀请码注册
- **服务器地址覆盖**：登录页可填 `http://...`，留空使用浏览器当前 origin
- **总览**：设备数、在线数、流量、最近设备/公告
- **设备管理**：新增/编辑/重置编号/设备码、匿名/设备码/接收连接开关
- **远程连接**：输入 12 位设备编号 + 设备码发起 WebRTC 控制
- **控制画布**：鼠标/键盘/滚轮/触屏事件 → DataChannel 控制指令
- **参数面板**：缩放、码率（对数）、帧率、编解码、防窥屏开关
- **虚拟键盘/鼠标/LR 键/滚轮**（移动）
- **文件传输**：分片 256KB / base64 编码
- **剪贴板/隐私屏**
- **公告**：列表 + 详情 + 强制阅读标记
- **账号**：改密、切语言
- **管理后台**（admin/super admin 可见）：
  - 用户管理（封禁/权限/重置密码）
  - 邀请码生成/吊销
  - 公告发布/编辑/作废
  - OTA 包上传/列表/删除
  - 服务器运行时统计（goroutine、内存、流量、ICE 等）

## 开发

```bash
cd web
npm install
npm run dev
# 默认 http://localhost:5173 ，代理 /api、/ws 到 127.0.0.1:8080
```

## 生产构建

```bash
npm run build
# 产物在 dist/，把 dist 整个目录复制到 ../server/web/ 即可由服务端托管
```

## 多语言

`src/i18n/zh-CN.json` / `src/i18n/en-US.json` 平铺键值。运行时热切换。首启动根据 `navigator.language` 自动选 `zh-CN` 或 `en-US`；登录成功后服务端返回的用户 `locale` 会覆盖本地。

## 与服务端的协议

详见 `web/api.md`。

---

# LinkALL Web (English)

Single-page application built with **Svelte 5 (Runes) + Vite 6 + Tailwind 3**. Bundles the **Controller** (remote session UI) and **Admin Console** (users/invites/announcements/OTA/server monitor). Pairs with the Go server (default `http://127.0.0.1:8080`).

## Features

- Sign in / Sign up (with invite code)
- Server URL override on the sign-in page (blank = current origin)
- Dashboard: device count, online count, traffic, recent devices & announcements
- Device management: add/edit/reset code & password, anon / device-code / accept-connection toggles
- Remote connect: enter a 12-char device code + password, then WebRTC
- Control canvas: mouse / keyboard / wheel / touch → DataChannel control commands
- Parameter panel: scale, bitrate (log), FPS, codec, privacy screen toggle
- Virtual keyboard / mouse / L-R buttons / wheel (mobile)
- File transfer: 256KB chunks, base64
- Clipboard sync, privacy screen
- Announcements: list + detail + forced-read flag
- Account: change password, switch language
- Admin (admin/super admin only):
  - Users (ban / role / reset password)
  - Invites (create / revoke)
  - Announcements (publish / edit / revoke)
  - OTA packages (upload / list / delete)
  - Server runtime stats (goroutines, memory, traffic, ICE)

## Develop

```bash
cd web
npm install
npm run dev
# http://localhost:5173 with /api & /ws proxied to 127.0.0.1:8080
```

## Production

```bash
npm run build
# Output in dist/. Copy into ../server/web/ to be served by the Go binary.
```

## i18n

`src/i18n/zh-CN.json` / `src/i18n/en-US.json` are flat key-value stores. Hot-switch at runtime. The initial locale is chosen from `navigator.language`. After sign-in, the server's `user.locale` overrides the browser default.

## API

See `web/api.md`.

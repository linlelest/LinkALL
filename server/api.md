# LinkALL Server API

> 完整 REST + WebSocket 信令协议。所有请求/响应均为 JSON；信令为 WebSocket 文本帧。

## 通用约定

- 基础路径：`/api`（REST），`/ws/signaling`（WS）
- 鉴权：`Authorization: Bearer <jwt>` 或 `Cookie: token=<jwt>`
- 错误响应统一格式：`{ "error": "中文错误消息" }`
- 成功：HTTP 200/201，返回业务 JSON
- 时间戳：Unix 秒
- 语言：`zh-CN` / `en-US`

## 公开端点

### `GET /api/health`
健康检查。

**响应** `200`
```json
{ "ok": true, "version": "1.0.0", "server_time": "http://127.0.0.1:8080" }
```

### `GET /api/config`
拉取运行期配置（ICE 服务器、官方地址）。

**响应** `200`
```json
{
  "public_url": "http://127.0.0.1:8080",
  "official_server": "http://127.0.0.1:8080",
  "max_sessions": 200,
  "idle_timeout_min": 30,
  "data_retention_days": 30,
  "invite_ttl_hours": 72,
  "ice_servers": [
    { "urls": "stun:stun.l.google.com:19302" }
  ]
}
```

### `GET /api/ota/check`
OTA 检查（公开）。支持 `?platform=win64&version=1.0.0`。

**响应** `200`
```json
{
  "has_update": true,
  "force_update": false,
  "platform": "win64",
  "version": "1.1.0",
  "channel": "stable",
  "release_notes": "## 新增\n- xxx",
  "min_supported_version": "1.0.0",
  "file_size": 9876543,
  "sha256": "abcd...",
  "signature": "ed25519:...",
  "download_url": "/api/ota/download/3",
  "created_at": 1717200000
}
```

### `GET /api/ota/download/:id`
下载 OTA 包。响应 `Content-Type: application/octet-stream`，附加响应头：
- `X-Checksum-SHA256`
- `X-Package-Version`
- `X-Package-Platform`

---

## 认证

### `POST /api/auth/login`
登录或注册（action 区分）。

**Body**
```json
{ "action": "login", "username": "alice", "password": "secret123" }
```
注册：
```json
{ "action": "register", "username": "bob", "password": "secret123", "invite_code": "12345678" }
```

**响应** `200`
```json
{ "token": "eyJhbGciOi...", "user": { "id": 1, "username": "alice", "is_admin": true, "is_super_admin": true, "locale": "zh-CN" } }
```

### `GET /api/auth/me` *(鉴权)*
返回当前用户。

### `POST /api/auth/password` *(鉴权)*
```json
{ "old_password": "x", "new_password": "y" }
```

### `POST /api/auth/locale` *(鉴权)*
```json
{ "locale": "en-US" }
```

### `POST /api/auth/logout` *(鉴权)*
无操作，JWT 无状态。客户端删除 token 即可。

---

## 设备

### `POST /api/devices/register`
设备首次注册（被控端使用）。

**Body**
```json
{
  "device_code": "ABCD1234EFGH",
  "device_password": "devicePass123",
  "name": "kevin-pc",
  "platform": "win64",
  "os_version": "10.0.22631",
  "app_version": "1.0.0",
  "allow_anonymous": true,
  "require_device_code": true,
  "accept_connections": true,
  "tag": "office",
  "notes": "前台开发机"
}
```

**响应** `200`：`{ "device": {...}, "token": "<device-jwt>" }`

### `POST /api/devices/login`
设备重新登录。
```json
{ "device_code": "...", "device_password": "..." }
```

### `GET /api/devices` *(鉴权)*
返回当前用户设备列表。管理员返回全部。

### `GET /api/devices/:id` *(鉴权)*

### `PATCH /api/devices/:id` *(鉴权)*
```json
{ "name": "客厅电脑", "allow_anonymous": false, "tag": "客厅", "notes": "" }
```

### `POST /api/devices/:id/reset-code` *(鉴权)*
```json
{ "new_code": "NEW1234CODE", "new_password": "newPass" }
```

### `DELETE /api/devices/:id` *(鉴权)*
无主设备仅超级管理员可删。

---

## 公告

### `GET /api/announcements` *(鉴权)*

### `GET /api/announcements/unread` *(鉴权)*

### `GET /api/announcements/:id` *(鉴权)*

### `POST /api/announcements/:id/read` *(鉴权)*

### `POST /api/announcements` *(管理员)*
```json
{
  "title": "v1.1.0 升级公告",
  "content_md": "## 重点\n- 新增 X\n- 修复 Y",
  "platform": "win64",
  "min_version": "1.0.0",
  "pinned": true,
  "force_read": false,
  "signature": "ed25519:..."
}
```

### `PATCH /api/announcements/:id` *(管理员)*

### `DELETE /api/announcements/:id` *(管理员)*

---

## OTA 包

### `GET /api/ota/list` *(鉴权)*

### `POST /api/ota/upload` *(管理员，multipart/form-data)*
字段：
- `file`: 包文件
- `platform`: `win64` / `linux-x86_64` / `android-arm64` / `web`
- `version`: `1.1.0`
- `channel`: `stable` / `beta` / `canary`
- `release_notes`: 文本
- `force_update`: `true` / `false`
- `min_supported_version`: 文本
- `signature`: 可选

### `PATCH /api/ota/:id` *(管理员)*

### `DELETE /api/ota/:id` *(管理员)*

---

## 邀请码

### `GET /api/invites` *(管理员)*

### `POST /api/invites` *(管理员)*
```json
{ "max_uses": 1, "ttl_hours": 72, "note": "给同事" }
```

### `DELETE /api/invites/:id` *(管理员)*

---

## 管理后台

### `GET /api/admin/stats` *(管理员)*
```json
{
  "users": 12,
  "devices": 27,
  "online": 3,
  "sessions": 1,
  "sessions_total": 88,
  "bytes_tx": 12345678,
  "bytes_rx": 87654321,
  "server_time": 1717200000,
  "go_version": "go1.22.3",
  "go_routines": 64,
  "go_mem_alloc": 4194304,
  "go_sys": 12582912,
  "go_heap_inuse": 8388608
}
```

### `GET /api/admin/users` *(管理员)*

### `PATCH /api/admin/users/:id` *(管理员)*
```json
{ "banned": true, "is_admin": true, "is_super_admin": false, "new_password": "abc12345" }
```

### `DELETE /api/admin/users/:id` *(超级管理员)*

---

## 信令 (WebSocket `/ws/signaling`)

### 协议
- 客户端 → 服务端：`{ "type": "<msg>", "data": { ... } }`
- 服务端 → 客户端：`{ "type": "<msg>", "from": "<peer-id>", "to": "<peer-id>", "data": {...}, "ts": <ms> }`

### 消息类型

| type | 方向 | data 字段 | 说明 |
|------|------|-----------|------|
| `hello` | C→S | `{kind, device_code, token, user_id?}` | 注册：kind=`controlled` 或 `controller` |
| `welcome` | S→C | `{id, kind}` | 注册成功返回 peer id |
| `offer` / `answer` | 双向 | `{sdp}` | WebRTC SDP |
| `ice` | 双向 | `{candidate}` | ICE candidate |
| `request` | C→S→Controlled | `{device_code, mode, user_id?}` | 控制器发起连接请求 |
| `request_ack` | Controlled→C | `{allowed, require_code?}` | 被控端确认（`once`/`permanent`/`denied`） |
| `connection_info` | S→C | `{session_id, relay, ice_servers[]}` | 服务端下发连接信息 |
| `cmd` | 双向 | JSON 控制指令 | 走 DataChannel 也走此信令通道转发 |
| `file_meta` / `file_ack` / `file_data` / `file_end` | 双向 | 文件分片协议 | 见下方 |
| `ping` / `pong` | 双向 | – | 心跳 |
| `error` | S→C | `{msg}` | 错误消息 |
| `online` | S→C | `{device_code, online}` | 设备上下线事件（订阅式，可选） |

### `hello` 示例
```json
{
  "type": "hello",
  "data": {
    "kind": "controlled",
    "device_code": "ABCD1234EFGH",
    "token": "<device-jwt>"
  }
}
```

### `request` 流程
1. 控制器 A 发 `request`，`to=<device_code>`，`data.mode="anonymous"`。
2. 服务端转发到被控端 B。
3. B 弹出确认框，回复 `request_ack`：`{allowed: "once"|"permanent"|"denied", require_code: "123456"}`。
4. 控制器 A 拿到 ack 后，发起 `offer`/`answer`/`ice` 进行 WebRTC 协商。

### `cmd` 控制指令协议
```json
{
  "type": "cmd",
  "data": {
    "op": "mouse" | "key" | "wheel" | "type" | "clip" | "file_send" | "privacy" | "config",
    ...
  }
}
```

- `mouse`：`{x, y, button, down, click_count}`
- `key`：`{code, down}`（code 为 Win VK 或 X11 keysym）
- `wheel`：`{dx, dy}`
- `type`：`{text}`
- `clip`：`{text}`
- `file_send`：`{path, size, name}` （后续接 file_meta/file_data）
- `privacy`：`{enabled}`
- `config`：`{scale, bitrate_kbps, fps, codec}`

### 文件分片
1. `file_meta`：`{transfer_id, name, size, sha256, total_chunks, chunk_size}`
2. `file_data`：`{transfer_id, offset, data(base64)}`，单片 256KB
3. `file_end`：`{transfer_id, sha256}`
4. 接收方任意时刻可发 `file_ack`：`{transfer_id, received_offset}`

---

# LinkALL Server API (English)

REST + WebSocket signaling protocol. All requests/responses are JSON; signaling uses WebSocket text frames.

## Common
- Base: `/api` (REST), `/ws/signaling` (WS)
- Auth: `Authorization: Bearer <jwt>` or `Cookie: token=<jwt>`
- Error: `{ "error": "Chinese error message" }`
- Timestamps: Unix seconds
- Locales: `zh-CN` / `en-US`

## Public

### `GET /api/health`
Health check.

### `GET /api/config`
Runtime config (ICE servers, official URL).

### `GET /api/ota/check`
Public OTA check. Query: `?platform=win64&version=1.0.0`.

### `GET /api/ota/download/:id`
Download package. Headers: `X-Checksum-SHA256`, `X-Package-Version`, `X-Package-Platform`.

## Auth

### `POST /api/auth/login`
Login or register. Use `action: "login"` or `action: "register"` with `invite_code`.

### `GET /api/auth/me` *(auth)*
### `POST /api/auth/password` *(auth)*
### `POST /api/auth/locale` *(auth)*
### `POST /api/auth/logout` *(auth)*

## Devices

### `POST /api/devices/register`
### `POST /api/devices/login`
### `GET /api/devices` *(auth)*
### `GET /api/devices/:id` *(auth)*
### `PATCH /api/devices/:id` *(auth)*
### `POST /api/devices/:id/reset-code` *(auth)*
### `DELETE /api/devices/:id` *(auth)*

## Announcements

### `GET /api/announcements` *(auth)*
### `GET /api/announcements/unread` *(auth)*
### `GET /api/announcements/:id` *(auth)*
### `POST /api/announcements/:id/read` *(auth)*
### `POST /api/announcements` *(admin)*
### `PATCH /api/announcements/:id` *(admin)*
### `DELETE /api/announcements/:id` *(admin)*

## OTA Packages

### `GET /api/ota/list` *(auth)*
### `POST /api/ota/upload` *(admin, multipart/form-data)*
### `PATCH /api/ota/:id` *(admin)*
### `DELETE /api/ota/:id` *(admin)*

## Invites

### `GET /api/invites` *(admin)*
### `POST /api/invites` *(admin)*
### `DELETE /api/invites/:id` *(admin)*

## Admin

### `GET /api/admin/stats` *(admin)*
### `GET /api/admin/users` *(admin)*
### `PATCH /api/admin/users/:id` *(admin)*
### `DELETE /api/admin/users/:id` *(super admin)*

## Signaling (WebSocket `/ws/signaling`)

### Protocol
- Client → Server: `{ "type": "<msg>", "data": {...} }`
- Server → Client: `{ "type": "<msg>", "from": "<peer-id>", "to": "<peer-id>", "data": {...}, "ts": <ms> }`

### Message types

| type | direction | data | description |
|------|-----------|------|-------------|
| `hello` | C→S | `{kind, device_code, token, user_id?}` | Register: kind=`controlled` / `controller` |
| `welcome` | S→C | `{id, kind}` | Server returns peer id |
| `offer` / `answer` | both | `{sdp}` | WebRTC SDP |
| `ice` | both | `{candidate}` | ICE candidate |
| `request` | C→S→Controlled | `{device_code, mode, user_id?}` | Controller asks to connect |
| `request_ack` | Controlled→C | `{allowed, require_code?}` | Allowed=`once`/`permanent`/`denied` |
| `connection_info` | S→C | `{session_id, relay, ice_servers[]}` | Connection metadata |
| `cmd` | both | control JSON | Same as DataChannel commands |
| `file_meta` / `file_ack` / `file_data` / `file_end` | both | file chunk protocol | see below |
| `ping` / `pong` | both | – | heartbeat |
| `error` | S→C | `{msg}` | Error message |

### `cmd` control protocol
```json
{ "type": "cmd", "data": { "op": "mouse|key|wheel|type|clip|file_send|privacy|config", ... } }
```
- `mouse`: `{x, y, button, down, click_count}`
- `key`: `{code, down}` (Win VK or X11 keysym)
- `wheel`: `{dx, dy}`
- `type`: `{text}`
- `clip`: `{text}`
- `file_send`: `{path, size, name}`
- `privacy`: `{enabled}`
- `config`: `{scale, bitrate_kbps, fps, codec}`

### File transfer
1. `file_meta`: `{transfer_id, name, size, sha256, total_chunks, chunk_size}`
2. `file_data`: `{transfer_id, offset, data(base64)}`, 256KB each
3. `file_end`: `{transfer_id, sha256}`
4. Receiver may reply with `file_ack`: `{transfer_id, received_offset}`

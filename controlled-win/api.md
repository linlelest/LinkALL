# LinkALL Hosted (Windows) — Tauri Command API

> 前端通过 `@tauri-apps/api/core` 的 `invoke()` 调用后端 `#[tauri::command]` 函数。  
> TypeScript 包装见 `src/lib/api.ts`。

## 服务端 / 本地配置

### `get_server() -> ServerConfig`
返回当前持久化的服务器配置（公网 URL / 官方 URL / ICE 列表）。

### `set_server(url: string) -> void`
更新服务器地址并写入本地 SQLite。

### `get_locale() -> string`
读取持久化语言（默认跟随系统）。

### `set_locale(locale: string) -> void`

## 设备

### `register_device(req) -> DeviceInfo`
首次注册。`req` 字段：
```ts
{ device_code: string, device_password: string, name?: string,
  platform?: string, os_version?: string, app_version?: string,
  allow_anonymous?: bool, require_device_code?: bool, accept_connections?: bool }
```
返回 `{ id, device_code, token, name, ... }` 并保存到本地。

### `login_device(req) -> DeviceInfo`
```ts
{ device_code: string, device_password: string }
```

### `get_device() -> DeviceInfo | null`
读取本地已登录设备。

### `update_flags(allow_anonymous, require_device_code, accept_connections) -> DeviceInfo`

### `reset_code(new_code, new_password) -> DeviceInfo`
`new_code` 留空时由服务端生成 12 位大写字符串。

### `logout_device() -> void`
清空本地设备记录。

## 状态

### `get_status() -> { running, signaling, last_error, screen_w, screen_h }`

### `start_service() -> void`
启动 WebSocket 信令 + 屏幕捕获循环（仅在已登录设备下生效）。

### `stop_service() -> void`

## 系统

### `set_autostart(enable: bool) -> void`
Win: 注册表 `Run` 键 / Linux: `~/.config/autostart/*.desktop`。

### `get_autostart() -> bool`

### `quit_app() -> void`

### `show_main() -> void`

### `respond_request(req_id, allowed) -> void`
对收到的连接请求做出回复。`allowed`: `"once" | "permanent" | "denied"`。  
非 denied 时会启动 `accept_request` 创建一个 WebRTC P2P 会话。

## 事件

通过 `@tauri-apps/api/event::listen` 订阅：

| event | payload | 说明 |
|-------|---------|------|
| `log` | `string` | 来自信令/抓屏/WebRTC 的日志 |
| `status` | `object` | 状态变化 |
| `connection_request` | `{ id, from, device_code, mode, ts }` | 收到新连接请求，UI 弹窗 |

## 信令消息（WebSocket `/ws/signaling`）

见 `../server/api.md` 的"信令"章节。被控端启动后会向服务端发送：

```json
{ "type": "hello", "data": { "kind": "controlled", "device_code": "...", "token": "<device-jwt>" } }
```

之后会收到 `request` / `offer` / `answer` / `ice` / `cmd` / `error` 等。

## 控制指令（cmd）

```json
{ "op": "mouse", "x": 12.3, "y": 78.9, "button": 0, "down": true }
{ "op": "key",   "code": "Enter", "down": true }
{ "op": "wheel", "dx": 0, "dy": -120 }
{ "op": "type",  "text": "hello" }
{ "op": "privacy", "enabled": true }
{ "op": "config", "scale": 1.0, "bitrate_kbps": 4096, "fps": 30, "codec": "h264" }
```

---

# LinkALL Hosted (English) — Tauri Command API

The frontend invokes these via `@tauri-apps/api/core::invoke`. The TS wrapper is in `src/lib/api.ts`.

## Server / Local config

### `get_server() -> ServerConfig`
Returns the persisted server config (public URL, official URL, ICE list).

### `set_server(url) -> void`
Persists the new server URL.

### `get_locale() / set_locale(...)`

## Device

### `register_device(req) -> DeviceInfo`
First-time registration. `req`:
```ts
{ device_code, device_password, name?, platform?, os_version?, app_version?,
  allow_anonymous?, require_device_code?, accept_connections? }
```

### `login_device({ device_code, device_password }) -> DeviceInfo`
### `get_device() -> DeviceInfo | null`
### `update_flags(allow_anonymous, require_device_code, accept_connections) -> DeviceInfo`
### `reset_code(new_code, new_password) -> DeviceInfo`
`new_code` blank → server generates a 12-char uppercase string.
### `logout_device() -> void`

## Status

### `get_status() -> { running, signaling, last_error, screen_w, screen_h }`
### `start_service() -> void`
### `stop_service() -> void`

## System

### `set_autostart(enable) / get_autostart()`
### `quit_app() / show_main()`
### `respond_request(req_id, allowed) -> void`
`allowed` ∈ `"once" | "permanent" | "denied"`. Non-denied creates a WebRTC P2P session.

## Events

Subscribe via `@tauri-apps/api/event::listen`:

| event | payload | description |
|-------|---------|-------------|
| `log` | `string` | log lines from signaling/screen/webrtc |
| `status` | `object` | status changes |
| `connection_request` | `{ id, from, device_code, mode, ts }` | incoming connection request |

## Signaling messages (WebSocket `/ws/signaling`)

See `../server/api.md`. On start, the host sends:
```json
{ "type": "hello", "data": { "kind": "controlled", "device_code": "...", "token": "<device-jwt>" } }
```

Then it receives `request` / `offer` / `answer` / `ice` / `cmd` / `error`.

## Control commands (`cmd`)

```json
{ "op": "mouse", "x": 12.3, "y": 78.9, "button": 0, "down": true }
{ "op": "key",   "code": "Enter", "down": true }
{ "op": "wheel", "dx": 0, "dy": -120 }
{ "op": "type",  "text": "hello" }
{ "op": "privacy", "enabled": true }
{ "op": "config", "scale": 1.0, "bitrate_kbps": 4096, "fps": 30, "codec": "h264" }
```

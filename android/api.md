# LinkALL Android — API 参考

> 本文件描述 Android 端 ↔ Go 服务端的所有接口契约。Android 端实现见 `app/src/main/java/com/linkall/app/data/api/ApiService.kt`；后端实现见 `../server/api.md`。

## 通用

- 基础 URL：通过 `Prefs.serverOverride` 覆盖；默认 `http://127.0.0.1:8080`
- 鉴权：登录后服务端返回的 JWT 自动注入 `Authorization: Bearer <token>`（`AuthInterceptor`）；设备登录使用另一条 device token
- 错误：`HTTPException` 透传给 UI；前端 catch 后显示在 Snackbar/Alert

## Auth

| Method | Path | 备注 |
|--------|------|------|
| POST | /api/auth/login | `LoginReq(action, username, password, invite_code?)` |
| GET | /api/auth/me |  |
| POST | /api/auth/password | `{old_password, new_password}` |
| POST | /api/auth/locale | `{locale}` |

## Devices

| Method | Path | 备注 |
|--------|------|------|
| POST | /api/devices/register | `RegisterDeviceReq(...)` |
| POST | /api/devices/login | `LoginDeviceReq(code, pw)` |
| GET | /api/devices | 列表 |
| GET | /api/devices/{id} | 详情 |
| PATCH | /api/devices/{id} | `UpdateDeviceReq(...)` |
| POST | /api/devices/{id}/reset-code | `ResetDeviceReq(new_code, new_password)` |
| DELETE | /api/devices/{id} |  |

## Announcements

| Method | Path | 备注 |
|--------|------|------|
| GET | /api/announcements?include_revoked=false |  |
| GET | /api/announcements/unread |  |
| GET | /api/announcements/{id} |  |
| POST | /api/announcements/{id}/read |  |

## OTA

| Method | Path | 备注 |
|--------|------|------|
| GET | /api/ota/check?platform=android-arm64&version=1.0.0 | 返回 `OtaInfo` |
| GET | /api/ota/list?include_revoked=false |  |

## Admin

| Method | Path | 备注 |
|--------|------|------|
| GET | /api/admin/stats | `AdminStats` |
| GET | /api/admin/users |  |
| PATCH | /api/admin/users/{id} | `AdminUpdateUserReq` |
| DELETE | /api/admin/users/{id} | 超级管理员 |
| GET | /api/invites |  |
| POST | /api/invites | `CreateInviteReq` |
| DELETE | /api/invites/{id} |  |

## 信令 (WebSocket `/ws/signaling`)

由 `WebRtcController`（控制端）和 `WebRtcHost`（被控端）通过 OkHttp WebSocket 客户端连接。

帧格式：`{ "type": "<msg>", "to": "<device_code|peer_id>", "data": { ... } }`

| type | 方向 | data 字段 |
|------|------|-----------|
| `hello` | C→S / Hosted→S | `{kind: "controlled"\|"controller", device_code, token}` |
| `welcome` | S→C | `{id, kind}` |
| `request` | C→S→Controlled | `{device_code, mode}` |
| `request_ack` | Controlled→C | `{allowed, require_code?}` |
| `offer` / `answer` | 双向 | `{sdp}` |
| `ice` | 双向 | `{candidate}` |
| `cmd` | 双向 | 控制指令（见下） |
| `file_meta` / `file_data` / `file_end` | 双向 | 256KB 分片 |
| `error` | S→C | `{msg}` |

## 控制指令 (cmd)

```json
{ "op": "mouse", "x": 12.3, "y": 78.9, "button": 0, "down": true }
{ "op": "key",   "code": "Enter", "down": true }
{ "op": "wheel", "dx": 0, "dy": -120 }
{ "op": "type",  "text": "hello" }
{ "op": "privacy", "enabled": true }
{ "op": "config", "scale": 1.0, "bitrate_kbps": 4096, "fps": 30, "codec": "h264" }
{ "op": "file_send", "transfer_id": "abc", "name": "demo.zip", "size": 1048576 }
{ "op": "file_data", "transfer_id": "abc", "offset": 0, "data": "<base64>" }
{ "op": "file_end",  "transfer_id": "abc", "sha256": "..." }
```

## 本地 Room 表

| 表 | 字段 |
|----|------|
| `device` | id, deviceCode, name, platform, osVersion, appVersion, allowAnonymous, requireDeviceCode, acceptConnections, lastIp, lastSeen, online |
| `announcement` | id, title, contentMd, pinned, forceRead, createdAt, updatedAt, revoked, read |

## 模块间调用

```
ui.HostedScreen ──> DeviceRepo ──> ApiService ──> HTTP
ui.HostedScreen ──> HostedServiceController ──> ScreenCaptureService ──> MediaProjection
ui.ControllerScreen ──> WebRtcController ──> OkHttp WebSocket ──> 信令 Hub
ui.ControllerScreen ──> VirtualInputOverlay (Compose 触控) ──> WebRtcController.sendMouse
```

---

# LinkALL Android — API (English)

This file documents the contracts between the Android client and the Go server. The Android implementation is at `app/src/main/java/com/linkall/app/data/api/ApiService.kt`. The server implementation is in `../server/api.md`.

## General
- Base URL: `Prefs.serverOverride` (default `http://127.0.0.1:8080`)
- Auth: JWT injected by `AuthInterceptor`. Device login uses a separate device token.

## Endpoints
(Same list as the Chinese section above.)

## Signaling
(Same protocol as the Chinese section.)

## Control commands
(Same protocol as the Chinese section.)

## Local Room tables
(See Chinese section.)

## Internal module wiring
(See Chinese section.)

# LinkALL Admin API

> 后台管理系统的全部后端契约。完整列表见 `../server/api.md`，本文件只摘录管理员相关端点。

## 用户管理

### `GET /api/admin/users` *(admin)*
返回全部用户列表。

**响应** `200`
```json
[
  { "id": 1, "username": "admin", "is_admin": true, "is_super_admin": true,
    "banned": false, "last_login_ip": "1.2.3.4", "last_login_at": 1717200000 }
]
```

### `PATCH /api/admin/users/:id` *(admin)*
```json
{ "banned": true, "is_admin": true, "is_super_admin": false, "new_password": "abc12345" }
```
- `banned` / `is_admin` / `is_super_admin` 任意组合
- `new_password` ≥ 6 字符，留空表示不改

### `DELETE /api/admin/users/:id` *(super admin)*

## 统计

### `GET /api/admin/stats` *(admin)*
```json
{
  "users": 12, "devices": 27, "online": 3, "sessions": 1,
  "sessions_total": 88,
  "bytes_tx": 12345678, "bytes_rx": 87654321,
  "server_time": 1717200000,
  "go_version": "go1.22.3", "go_routines": 64,
  "go_mem_alloc": 4194304, "go_sys": 12582912, "go_heap_inuse": 8388608
}
```

## 邀请码

### `GET /api/invites` *(admin)*
```json
[
  { "id": 1, "code": "12345678", "max_uses": 1, "used_count": 0,
    "ttl_hours": 72, "expires_at": 1717286400, "revoked": false, "note": "给 kevin" }
]
```

### `POST /api/invites` *(admin)*
```json
{ "max_uses": 1, "ttl_hours": 72, "note": "" }
```

### `DELETE /api/invites/:id` *(admin)*

## 公告

### `GET /api/announcements?include_revoked=true` *(admin/普通登录用户均可)*
### `POST /api/announcements` *(admin)*
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
### `PATCH /api/announcements/:id` *(admin)*
### `DELETE /api/announcements/:id` *(admin)*

## OTA

### `GET /api/ota/list?include_revoked=true` *(admin/普通登录用户均可)*

### `POST /api/ota/upload` *(admin，multipart/form-data)*
字段：
- `file`: 包文件
- `platform`: `win64` / `linux-x86_64` / `android-arm64` / `web`
- `version`: `1.1.0`
- `channel`: `stable` / `beta` / `canary`
- `release_notes`: 文本
- `force_update`: `true` / `false`
- `min_supported_version`: 文本
- `signature`: 可选 Ed25519 签名

服务端自动计算 SHA-256，并返回 `OTAPackage` 元数据。

### `PATCH /api/ota/:id` *(admin)*
### `DELETE /api/ota/:id` *(admin)*

---

# LinkALL Admin API (English)

The admin console uses the same routes documented in `../server/api.md`. The endpoints above are the most relevant for admin operations.

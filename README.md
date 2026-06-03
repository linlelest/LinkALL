# LinkALL

> 一个轻量级、跨平台的远程控制系统。  
> A lightweight, cross-platform remote control system.

**5 个端 · 一套协议 · 零费用开源**

| 端 | 角色 | 技术栈 |
|----|------|--------|
| [`server/`](./server/) | 后端服务（单二进制） | Go 1.22+ / Fiber v2 / SQLite / WebSocket |
| [`web/`](./web/) | 网页控制端 + 管理后台 | Svelte 5 / Vite 6 / Tailwind 3 / TypeScript |
| [`controlled-win/`](./controlled-win/) | Windows 桌面被控端 | Tauri 2 / Rust 1.85 / scrap / enigo / webrtc-rs |
| [`android/`](./android/) | Android 三合一 App（被控 + 控制 + 管理） | Kotlin 2.1 / Compose / WebRTC SDK 125 / 最低 Android 7.0 (API 24) |
| [`admin/`](./admin/) | 后台管理（与 `web/` 共享 `#/admin/*`） | 文档索引 |

## 特性

- 🔐 **安全**：Argon2id 密码哈希 + JWT (HS256) + 设备 token + RBAC
- 🌐 **跨平台**：Windows / macOS / Linux / Android / 任意浏览器
- 🚀 **P2P 直连**：WebRTC MediaStream + DataChannel，自带 TURN 预留
- 🪶 **极轻量**：Go 单二进制 15MB；Tauri 桌面端 8MB；Android APK 18MB
- 🗂️ **零依赖部署**：SQLite 单文件；modernc 驱动纯 Go，零 CGO
- 🌍 **中英双语**：所有端 UI + 文档全量 i18n
- 🔧 **可扩展**：JSON 信令协议 + DataChannel 指令，命令可自定义

## 5 分钟上手

### 1. 启动服务端
```bash
cd server
cp .env.example .env
# 改 JWT_SECRET 为 32+ 随机字符串
go build -trimpath -ldflags "-s -w" -o linkall-server ./cmd/server
./linkall-server
# 首次启动会在终端引导创建超级管理员
```

### 2. 构建并部署网页
```bash
cd web
npm install
npm run build
# 将 dist/* 复制到 ../server/web/
```

### 3. Windows 被控端
```bash
cd controlled-win
npm install
npx tauri icon src-tauri/icons/icon.png  # 一次性图标生成
npm run tauri build
```

### 4. Android
```bash
cd android
./gradlew :app:assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk
```

## 文档

| 文档 | 说明 |
|------|------|
| [`readmeforai.md`](./readmeforai.md) | **给 AI 看的完整项目总览**（技术栈 / 进度 / TODO） |
| [`server/README.md`](./server/README.md) + [`server/api.md`](./server/api.md) | 后端使用 + API 契约 |
| [`web/README.md`](./web/README.md) + [`web/api.md`](./web/api.md) | 网页前端使用 + WebSocket / WebRTC 协议 |
| [`controlled-win/README.md`](./controlled-win/README.md) + [`controlled-win/api.md`](./controlled-win/api.md) | 桌面被控端使用 + Tauri IPC 命令 |
| [`android/README.md`](./android/README.md) + [`android/api.md`](./android/api.md) | Android App 使用 + Compose API |
| [`admin/README.md`](./admin/README.md) + [`admin/api.md`](./admin/api.md) | 后台管理（指向 `web/#/admin/*`） |

## 截图（待补）

> 运行截图：待 v1.0 发布时补

## 协议

MIT © 2026 linlelest — 见 [LICENSE](./LICENSE)

## 反馈

- GitHub Issues
- Email: 见仓库

---

# LinkALL (English)

**5 clients · 1 protocol · open-source & free**

| Component | Role | Stack |
|-----------|------|-------|
| [`server/`](./server/) | Backend (single binary) | Go 1.22+ / Fiber v2 / SQLite / WebSocket |
| [`web/`](./web/) | Web client + admin console | Svelte 5 / Vite 6 / Tailwind 3 / TypeScript |
| [`controlled-win/`](./controlled-win/) | Windows host | Tauri 2 / Rust 1.85 / scrap / enigo / webrtc-rs |
| [`android/`](./android/) | Android 3-in-1 App (host + controller + admin) | Kotlin 2.1 / Compose / WebRTC SDK 125 / Min Android 7.0 (API 24) |
| [`admin/`](./admin/) | Admin (shares `web/#/admin/*`) | docs only |

## Features
- 🔐 Argon2id + JWT + device token + RBAC
- 🌐 Windows / macOS / Linux / Android / any browser
- 🚀 P2P via WebRTC MediaStream + DataChannel
- 🪶 Go binary 15MB / Tauri 8MB / Android 18MB
- 🗂️ SQLite single file, zero CGO
- 🌍 zh-CN / en-US throughout
- 🔧 JSON signaling + customizable DataChannel commands

## Quick start
Same as Chinese section. See `readmeforai.md` for the full AI-friendly project overview.

## License
MIT © 2026 linlelest — see [LICENSE](./LICENSE)

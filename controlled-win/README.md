# LinkALL Hosted (Windows 被控端)

> 基于 **Tauri 2 + Rust 1.85** 的 Windows 桌面被控端。主进程 Rust 极轻（<5MB），UI 用 Svelte 5 + Tailwind。负责：
> - 注册/登录 LinkALL 设备
> - 接收 WebRTC P2P 控制会话
> - 通过 DXGI（`scrap`）抓屏 → 推流（默认 H.264 Baseline）
> - 接收 `cmd` 指令 → 通过 `enigo` 注入键鼠
> - 防窥屏：覆盖一个 `WS_EX_TOPMOST` 全屏黑窗口
> - 开机自启：注册表 `Run` 键
> - 托盘：显示主窗口/启停服务/退出
> - 本地 SQLite 存储设备凭据、设置

## 编译

```bash
# 1) 安装 Rust 1.85+ 与 Node 20+
rustc --version
node -v

# 2) 在 controlled-win/ 下：
npm install
# 生成图标（需先放置一个 1024x1024 的 icon.png 到 src-tauri/icons/）
npx tauri icon src-tauri/icons/icon.png
# 构建
npm run tauri build
# 产物：src-tauri/target/release/bundle/msi/*.msi
#        src-tauri/target/release/bundle/nsis/*.exe
```

## 开发模式

```bash
npm install
npm run tauri dev
```

## 配置

启动后通过主窗口"登录"或"注册"。  
服务器地址可在"登录"卡片顶部填写（如 `http://192.168.1.10:8080`），保存后即写入本地 SQLite。

## 关键模块

| 文件 | 作用 |
|------|------|
| `src-tauri/src/lib.rs` | Tauri 入口、托盘、命令注册 |
| `src-tauri/src/state.rs` | 全局 `AppState`（DB、HTTP、Signaling、Status） |
| `src-tauri/src/db.rs` | 本地 SQLite（rusqlite）|
| `src-tauri/src/server_api.rs` | 服务端 HTTP 客户端（reqwest）|
| `src-tauri/src/signaling.rs` | WebSocket 信令客户端（tokio-tungstenite）|
| `src-tauri/src/webrtc_host.rs` | WebRTC P2P 主机端（webrtc-rs）|
| `src-tauri/src/screen.rs` | DXGI 抓屏 + config 同步 |
| `src-tauri/src/input.rs` | SendInput 键鼠注入（enigo）|
| `src-tauri/src/privacy.rs` | 防窥屏（topmost 黑窗口）|
| `src-tauri/src/autostart.rs` | 开机自启（注册表 / autostart .desktop）|
| `src-tauri/src/cmd.rs` | Tauri commands |
| `src/App.svelte` | 主窗口 UI（登录/运行状态/控制）|

## 关键依赖

- `tauri 2`
- `webrtc 0.12`（`webrtc-rs`，纯 Rust WebRTC 栈）
- `scrap 0.5`（DXGI Desktop Duplication）
- `enigo 0.3`（跨平台键鼠）
- `tokio-tungstenite 0.24`（WS 客户端）
- `reqwest 0.12`（HTTP）
- `rusqlite 0.32`（本地 DB）
- `auto-launch 0.5`（开机自启）
- `windows 0.58`（Win32 绑定）

> **重要**：H.264 硬件/软件编码接入点见 `src-tauri/src/webrtc_host.rs` 的 `encode_frame` 占位函数。  
> 真实工程中应在此处接入 `windows::Media::MediaFoundation` 的 H.264 MFT 或 NVENC；当前默认以原始 BGRA 数据作为占位，浏览器端无法直接解码。后续替换为 H.264 AnnexB 即可立即可用。

## 已知 TODO

- [ ] 接 MFT / NVENC 做 H.264 硬编
- [ ] Linux X11/Wayland 截屏（`scrap` 暂未支持 Wayland）
- [ ] 软编 fallback：集成 `openh264` / `x264`
- [ ] 多显示器选择
- [ ] 自动重连与降级

详见 `../readmeforai.md`。

---

# LinkALL Hosted (English)

Tauri 2 + Rust 1.85 desktop controlled end for Windows. The main process is a tiny Rust binary (<5MB) and the UI is Svelte 5 + Tailwind. Responsibilities:
- Register / log in a LinkALL device
- Accept WebRTC P2P control sessions
- DXGI screen capture (`scrap`) → push stream (H.264 Baseline by default)
- Receive `cmd` messages → `enigo` input injection
- Privacy screen: full-screen `WS_EX_TOPMOST` black window
- Auto-start: registry `Run` key
- System tray: show / toggle service / quit
- Local SQLite for device creds & settings

## Build

```bash
# Rust 1.85+ and Node 20+ required
cd controlled-win
npm install
npx tauri icon src-tauri/icons/icon.png    # generate icon set from a 1024x1024 PNG
npm run tauri build
# Outputs: src-tauri/target/release/bundle/{msi,nsis}/*
```

## Dev

```bash
npm install
npm run tauri dev
```

## Configuration

Use the main window to log in / register. Set the server URL on the same card (e.g. `http://192.168.1.10:8080`); it is persisted to local SQLite.

## Key modules

| file | purpose |
|------|---------|
| `src-tauri/src/lib.rs` | Tauri entry, tray, command registration |
| `src-tauri/src/state.rs` | global `AppState` (DB, HTTP, Signaling, Status) |
| `src-tauri/src/db.rs` | local SQLite (rusqlite) |
| `src-tauri/src/server_api.rs` | server HTTP client (reqwest) |
| `src-tauri/src/signaling.rs` | WS signaling client (tokio-tungstenite) |
| `src-tauri/src/webrtc_host.rs` | WebRTC P2P host (webrtc-rs) |
| `src-tauri/src/screen.rs` | DXGI capture + config sync |
| `src-tauri/src/input.rs` | SendInput injection (enigo) |
| `src-tauri/src/privacy.rs` | privacy screen (topmost black window) |
| `src-tauri/src/autostart.rs` | auto-start (registry / autostart .desktop) |
| `src-tauri/src/cmd.rs` | Tauri commands |
| `src/App.svelte` | main window UI (login / runtime / control) |

## Key deps
- `tauri 2`
- `webrtc 0.12` (webrtc-rs, pure Rust WebRTC)
- `scrap 0.5` (DXGI Desktop Duplication)
- `enigo 0.3` (cross-platform input)
- `tokio-tungstenite 0.24` (WS client)
- `reqwest 0.12` (HTTP)
- `rusqlite 0.32` (local DB)
- `auto-launch 0.5` (auto-start)
- `windows 0.58` (Win32)

> **IMPORTANT**: the H.264 hardware/software encode hook lives in `src-tauri/src/webrtc_host.rs::encode_frame` (placeholder). In production wire MFT / NVENC / `openh264` here. Browsers cannot decode raw BGRA; the placeholder data is not a valid stream.

## TODO
- [ ] MFT / NVENC H.264 encode
- [ ] Linux X11/Wayland capture (Wayland not yet supported by `scrap`)
- [ ] Software fallback (`openh264` / `x264`)
- [ ] Multi-monitor selection
- [ ] Auto-reconnect / graceful degradation

See `../readmeforai.md`.

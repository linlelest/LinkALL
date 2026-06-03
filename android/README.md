# LinkALL Android (被控 + 控制 + 管理 三合一 App)

> Kotlin + Jetpack Compose + WebRTC 的原生 Android App。三 Tab 架构：`被控` / `控制` / `管理`。
> 与 Go 服务端配套。最低 Android 7.0 (API 24)，目标 Android 15 (API 35)。

## 功能

### 被控端 (Hosted)
- 12 位设备编号 + 设备码注册 / 登录
- 安全开关：允许匿名 / 需要设备码 / 接受连接
- 重置设备编号 / 设备码
- 退出登录并清空本机数据
- 启动前台 `ScreenCaptureService` 抓屏（MediaProjection 框架）
- 引导开启无障碍服务（用于键鼠注入）
- 引导电池优化白名单、开机自启
- 收到连接请求时弹窗（仅本次 / 永久允许 / 拒绝）

### 控制端 (Controller)
- 输入 12 位设备编号发起 WebRTC 远程连接
- `SurfaceViewRenderer` 渲染视频流，双指/触摸 → mouse 事件
- 虚拟键盘（`Modifier.pointerInput` 拼装）
- 虚拟鼠标 / 滚轮 / 左右键浮动按钮
- 参数面板：缩放、码率、帧率、编解码、防窥屏开关
- 通过 SAF 发送 / 接收文件（DataChannel 分片 256KB）

### 管理端 (Admin)
- 账号信息、改密、切换语言
- 设备列表
- 公告中心（MD 渲染）
- OTA 检查
- 服务器运行时统计

## 模块

| 目录 | 作用 |
|------|------|
| `app/src/main/java/com/linkall/app/LinkALLApp.kt` | Application / Koin 启动 |
| `app/src/main/java/com/linkall/app/ui/MainActivity.kt` | 唯一 Activity |
| `app/src/main/java/com/linkall/app/ui/nav/RootScaffold.kt` | 底部三 Tab + 内容切换 |
| `app/src/main/java/com/linkall/app/ui/login/LoginScreen.kt` | 登录 / 注册 |
| `app/src/main/java/com/linkall/app/ui/hosted/HostedScreen.kt` | 被控端 UI |
| `app/src/main/java/com/linkall/app/ui/controller/ControllerScreen.kt` | 控制端 UI |
| `app/src/main/java/com/linkall/app/ui/admin/AdminScreen.kt` | 管理端 UI |
| `app/src/main/java/com/linkall/app/ui/theme/Theme.kt` | Material 3 主题 |
| `app/src/main/java/com/linkall/app/ui/components/Common.kt` | 通用控件 |
| `app/src/main/java/com/linkall/app/data/api/ApiService.kt` | Retrofit API |
| `app/src/main/java/com/linkall/app/data/db/AppDatabase.kt` | Room 本地 DB |
| `app/src/main/java/com/linkall/app/data/repo/*.kt` | Repository 层 |
| `app/src/main/java/com/linkall/app/hosted/*.kt` | 截屏服务 / 无障碍 / 自启 / WebRTC 主机端 |
| `app/src/main/java/com/linkall/app/controller/*.kt` | WebRTC 客户端 / 虚拟输入 |
| `app/src/main/java/com/linkall/app/di/AppModule.kt` | Koin 依赖注入 |
| `app/src/main/java/com/linkall/app/util/Prefs.kt` | 轻量 KV 持久化 |

## 编译

要求 JDK 17、AGP 8.7+、Android SDK 35。

```bash
cd android
./gradlew :app:assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk
```

> 首次构建会自动下载 Kotlin / Compose / WebRTC 等依赖，国内可配代理。

## 真机调试

1. 设置 → 开发者选项 → USB 调试 开
2. 连接后 `adb devices` 确认
3. `./gradlew :app:installDebug`

## 权限

- INTERNET / ACCESS_NETWORK_STATE
- FOREGROUND_SERVICE + FOREGROUND_SERVICE_MEDIA_PROJECTION / _SPECIAL_USE（API 34+）
- POST_NOTIFICATIONS
- RECEIVE_BOOT_COMPLETED / WAKE_LOCK
- REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
- SYSTEM_ALERT_WINDOW
- `BIND_ACCESSIBILITY_SERVICE`（在 Manifest 中通过 `<service>` 注册）

## 已知 TODO

- [ ] 真正的 MediaProjection 截屏 + WebRTC VideoTrack 推流（在 `ScreenCaptureService` 内完成推帧到 `WebRtcHost`）
- [ ] AccessibilityService 内 `dispatchKeyEvent` 通过自定义 IME 实现键入
- [ ] 各厂商 ROM 自启引导页面映射表补充
- [ ] 文件分片下载进度持久化、断点续传
- [ ] Compose 多语言热切换
- [ ] 强制更新拦截 Activity 返回

详见 `../readmeforai.md`。

---

# LinkALL Android (English)

Native Android client (Kotlin + Jetpack Compose + WebRTC). Three-tab: **Host** / **Control** / **Admin**. Pairs with the Go server. Min Android 7.0 (API 24), target Android 15 (API 35).

## Features

### Host
- 12-char device code + device-password register / sign-in
- Security toggles: allow anonymous / require code / accept connections
- Reset device code / password
- Logout (clears local data)
- Foreground `ScreenCaptureService` (MediaProjection)
- Accessibility service enablement guide (for key/mouse injection)
- Battery optimisation & autostart guides
- Connection request dialog (once / always / deny)

### Control
- 12-char device code input
- `SurfaceViewRenderer` rendering
- Touch overlay → mouse events
- Virtual keyboard / mouse / wheel / L-R buttons
- Param panel: scale / bitrate / FPS / codec / privacy
- File transfer via SAF (DataChannel 256KB chunks)

### Admin
- Account info / change password / switch language
- Device list
- Announcements (Markdown)
- OTA check
- Server runtime stats

## Modules

(see Chinese section above for the full table)

## Build

Requires JDK 17, AGP 8.7+, Android SDK 35.

```bash
cd android
./gradlew :app:assembleDebug
# Output: app/build/outputs/apk/debug/app-debug.apk
```

## Debug on device

1. Developer options → USB debugging on
2. `adb devices`
3. `./gradlew :app:installDebug`

## Permissions

(See Chinese section for the full list.)

## TODO
- Real MediaProjection frame capture → WebRTC VideoTrack (`ScreenCaptureService` + `WebRtcHost`)
- KeyEvent injection via custom IME
- More vendor ROM autostart pages
- File chunk download persistence / resume
- Hot locale switch in Compose
- Forced update intercepting Activity back

See `../readmeforai.md`.

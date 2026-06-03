// Tauri 命令（前端 invoke 入口）
use crate::config::ServerConfig;
use crate::db::DeviceRow;
use crate::state::AppState;
use base64::{engine::general_purpose, Engine as _};
use serde::Serialize;
use std::sync::Arc;
use tauri::{Manager, State};

#[derive(Serialize)]
pub struct DeviceInfo {
    pub id: i64,
    pub device_code: String,
    pub name: Option<String>,
    pub platform: Option<String>,
    pub os_version: Option<String>,
    pub app_version: Option<String>,
    pub allow_anonymous: bool,
    pub require_device_code: bool,
    pub accept_connections: bool,
    pub last_ip: Option<String>,
    pub last_seen: Option<i64>,
    pub created_at: Option<i64>,
    pub online: bool,
    pub token: String,
}

impl From<DeviceRow> for DeviceInfo {
    fn from(d: DeviceRow) -> Self {
        Self {
            id: d.id, device_code: d.device_code, name: d.name, platform: d.platform,
            os_version: d.os_version, app_version: d.app_version,
            allow_anonymous: d.allow_anonymous, require_device_code: d.require_device_code,
            accept_connections: d.accept_connections, last_ip: d.last_ip, last_seen: d.last_seen,
            created_at: d.created_at, online: d.online, token: d.token,
        }
    }
}

#[tauri::command]
pub fn get_server(state: State<'_, Arc<AppState>>) -> ServerConfig {
    state.server.lock().clone()
}

#[tauri::command]
pub fn set_server(state: State<'_, Arc<AppState>>, url: String) -> Result<(), String> {
    let u = url.trim().trim_end_matches('/').to_string();
    if !(u.starts_with("http://") || u.starts_with("https://")) {
        return Err("URL 必须以 http:// 或 https:// 开头".into());
    }
    state.rebuild_client(&u);
    state.db.kv_set("server_url", &u).map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
pub fn get_locale(state: State<'_, Arc<AppState>>) -> String {
    state.db.kv_get("locale").unwrap_or_else(|| {
        // 简单按系统语言
        std::env::var("LANG").unwrap_or_else(|_| "zh-CN".into()).split('.').next().unwrap_or("zh-CN").replace('_', "-")
    })
}

#[tauri::command]
pub fn set_locale(state: State<'_, Arc<AppState>>, locale: String) -> Result<(), String> {
    state.db.kv_set("locale", &locale).map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
pub async fn register_device(
    state: State<'_, Arc<AppState>>,
    req: serde_json::Value,
) -> Result<DeviceInfo, String> {
    let client = state.client.lock().clone();
    let base = state.current_server();
    let mut body = req.clone();
    if body.get("platform").is_none() { body["platform"] = serde_json::json!("win64"); }
    if body.get("app_version").is_none() { body["app_version"] = serde_json::json!(env!("CARGO_PKG_VERSION")); }
    if body.get("os_version").is_none() {
        body["os_version"] = serde_json::json!(std::env::consts::OS);
    }
    let device_code = body.get("device_code").and_then(|v| v.as_str()).unwrap_or("").to_string();
    let device_password = body.get("device_password").and_then(|v| v.as_str()).unwrap_or("").to_string();
    if device_code.is_empty() || device_password.is_empty() {
        return Err("device_code 与 device_password 必填".into());
    }
    let resp: serde_json::Value = client.register(body).await.map_err(|e| e.to_string())?;
    let token = resp.get("token").and_then(|v| v.as_str()).unwrap_or("").to_string();
    let d = resp.get("device").cloned().unwrap_or_default();
    let id = d.get("id").and_then(|v| v.as_i64()).unwrap_or(0);
    let row = DeviceRow {
        id,
        device_code: device_code.to_uppercase(),
        device_password,
        token: token.clone(),
        name: d.get("name").and_then(|v| v.as_str()).map(String::from),
        platform: Some("win64".into()),
        os_version: Some(std::env::consts::OS.into()),
        app_version: Some(env!("CARGO_PKG_VERSION").into()),
        allow_anonymous: d.get("allow_anonymous").and_then(|v| v.as_bool()).unwrap_or(true),
        require_device_code: d.get("require_device_code").and_then(|v| v.as_bool()).unwrap_or(true),
        accept_connections: d.get("accept_connections").and_then(|v| v.as_bool()).unwrap_or(true),
        last_ip: d.get("last_ip").and_then(|v| v.as_str()).map(String::from),
        last_seen: d.get("last_seen").and_then(|v| v.as_i64()),
        created_at: d.get("created_at").and_then(|v| v.as_i64()),
        online: true,
    };
    state.db.device_put(&row).map_err(|e| e.to_string())?;
    *state.device.lock() = Some(row.clone());
    let _ = base;
    Ok(row.into())
}

#[tauri::command]
pub async fn login_device(
    state: State<'_, Arc<AppState>>,
    req: serde_json::Value,
) -> Result<DeviceInfo, String> {
    let client = state.client.lock().clone();
    let device_code = req.get("device_code").and_then(|v| v.as_str()).unwrap_or("").to_uppercase();
    let device_password = req.get("device_password").and_then(|v| v.as_str()).unwrap_or("").to_string();
    let resp: serde_json::Value = client.login(serde_json::json!({
        "device_code": device_code, "device_password": device_password
    })).await.map_err(|e| e.to_string())?;
    let token = resp.get("token").and_then(|v| v.as_str()).unwrap_or("").to_string();
    let d = resp.get("device").cloned().unwrap_or_default();
    let id = d.get("id").and_then(|v| v.as_i64()).unwrap_or(0);
    let row = DeviceRow {
        id,
        device_code,
        device_password,
        token,
        name: d.get("name").and_then(|v| v.as_str()).map(String::from),
        platform: d.get("platform").and_then(|v| v.as_str()).map(String::from),
        os_version: d.get("os_version").and_then(|v| v.as_str()).map(String::from),
        app_version: d.get("app_version").and_then(|v| v.as_str()).map(String::from),
        allow_anonymous: d.get("allow_anonymous").and_then(|v| v.as_bool()).unwrap_or(true),
        require_device_code: d.get("require_device_code").and_then(|v| v.as_bool()).unwrap_or(true),
        accept_connections: d.get("accept_connections").and_then(|v| v.as_bool()).unwrap_or(true),
        last_ip: d.get("last_ip").and_then(|v| v.as_str()).map(String::from),
        last_seen: d.get("last_seen").and_then(|v| v.as_i64()),
        created_at: d.get("created_at").and_then(|v| v.as_i64()),
        online: true,
    };
    state.db.device_put(&row).map_err(|e| e.to_string())?;
    *state.device.lock() = Some(row.clone());
    Ok(row.into())
}

#[tauri::command]
pub fn get_device(state: State<'_, Arc<AppState>>) -> Option<DeviceInfo> {
    state.device.lock().clone().map(Into::into)
}

#[tauri::command]
pub async fn update_flags(
    state: State<'_, Arc<AppState>>,
    allow_anonymous: bool,
    require_device_code: bool,
    accept_connections: bool,
) -> Result<DeviceInfo, String> {
    let dev = state.device.lock().clone().ok_or("未登录")?;
    let client = state.client.lock().clone();
    let _ = client.update_flags(dev.id, &dev.token, serde_json::json!({
        "allow_anonymous": allow_anonymous,
        "require_device_code": require_device_code,
        "accept_connections": accept_connections,
    })).await.map_err(|e| e.to_string())?;
    let mut n = dev;
    n.allow_anonymous = allow_anonymous;
    n.require_device_code = require_device_code;
    n.accept_connections = accept_connections;
    state.db.device_put(&n).map_err(|e| e.to_string())?;
    *state.device.lock() = Some(n.clone());
    Ok(n.into())
}

#[tauri::command]
pub async fn reset_code(
    state: State<'_, Arc<AppState>>,
    new_code: String,
    new_password: String,
) -> Result<DeviceInfo, String> {
    let dev = state.device.lock().clone().ok_or("未登录")?;
    let client = state.client.lock().clone();
    // 留空则服务端自动生成
    let body = serde_json::json!({ "new_code": new_code, "new_password": new_password });
    let resp: serde_json::Value = client.reset_code(dev.id, &dev.token, body).await.map_err(|e| e.to_string())?;
    let new_code = resp.get("device_code").and_then(|v| v.as_str()).map(String::from).unwrap_or(dev.device_code.clone());
    let mut n = dev;
    n.device_code = new_code;
    n.device_password = new_password;
    state.db.device_put(&n).map_err(|e| e.to_string())?;
    *state.device.lock() = Some(n.clone());
    Ok(n.into())
}

#[tauri::command]
pub fn logout_device(state: State<'_, Arc<AppState>>) -> Result<(), String> {
    state.db.device_clear().map_err(|e| e.to_string())?;
    *state.device.lock() = None;
    Ok(())
}

#[tauri::command]
pub fn get_status(state: State<'_, Arc<AppState>>) -> serde_json::Value {
    let s = state.snapshot_status();
    serde_json::json!({
        "running": s.running,
        "signaling": s.signaling,
        "last_error": s.last_error,
        "screen_w": s.screen_w,
        "screen_h": s.screen_h,
    })
}

#[tauri::command]
pub async fn start_service(
    state: State<'_, Arc<AppState>>,
    app: tauri::AppHandle,
) -> Result<(), String> {
    let st = state.inner().clone();
    crate::state::start_service(st, app).await.map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn stop_service(state: State<'_, Arc<AppState>>) -> Result<(), String> {
    crate::state::stop_service(state.inner().clone()).await;
    Ok(())
}

#[tauri::command]
pub fn set_autostart(state: State<'_, Arc<AppState>>, enable: bool) -> Result<(), String> {
    let _ = state; // autostart 独立
    crate::autostart::set(enable).map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
pub fn get_autostart() -> bool {
    crate::autostart::get().unwrap_or(false)
}

#[tauri::command]
pub fn quit_app(app: tauri::AppHandle) {
    app.exit(0);
}

#[tauri::command]
pub fn show_main(app: tauri::AppHandle) {
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.show();
        let _ = w.set_focus();
    }
}

#[tauri::command]
pub async fn respond_request(
    state: State<'_, Arc<AppState>>,
    app: tauri::AppHandle,
    req_id: String,
    allowed: String,
) -> Result<(), String> {
    if let Some(sig) = state.signaling.lock().clone() {
        let resp = serde_json::json!({
            "type": "request_ack",
            "to": req_id,
            "data": { "allowed": allowed }
        });
        sig.send(resp);
    }
    if allowed == "once" || allowed == "permanent" {
        // 启动 WebRTC 会话前先确保 ICE servers 最新（含 TURN）
        state.refresh_ice_servers().await;
        // 启动 WebRTC 会话
        let st = state.inner().clone();
        if let Err(e) = crate::webrtc_host::accept_request(st, app, req_id, "anonymous".into()).await {
            return Err(e.to_string());
        }
    }
    Ok(())
}

#[tauri::command]
pub async fn get_ice_servers(state: State<'_, Arc<AppState>>) -> Result<Vec<crate::config::IceServer>, String> {
    // 拉一次最新，避免用旧 TURN 凭据
    state.refresh_ice_servers().await;
    Ok(state.server.lock().ice_servers.clone())
}

#[tauri::command]
pub fn list_displays() -> Vec<crate::screen::DisplayInfo> {
    crate::screen::list_displays()
}

#[tauri::command]
pub fn select_display(idx: usize) {
    crate::screen::select_display(idx);
}

#[tauri::command]
pub fn get_selected_display() -> usize {
    crate::screen::selected_display()
}

// === 日志 / 崩溃上报 ===

#[tauri::command]
pub async fn report_crash(
    state: State<'_, Arc<AppState>>,
    level: String,
    message: String,
    stack: Option<String>,
    source: Option<String>,
) -> Result<(), String> {
    let l = match crate::logger::global() {
        Some(l) => l,
        None => return Err("logger not initialised".into()),
    };
    // 先写本地
    l.log(crate::logger::Level::from_str(&level), &message, stack.clone());
    // 上报到服务端
    let device_code = state.db.device_get().map(|d| d.device_code).unwrap_or_default();
    let body = serde_json::json!({
        "device_code": device_code,
        "platform": "windows",
        "app_version": l.app_version(),
        "level": level,
        "source": source,
        "message": message,
        "stack": stack,
    });
    let url = format!("{}/api/crash", state.current_server());
    let http = state.client.lock().http.clone();
    let _ = http
        .post(&url)
        .json(&body)
        .send()
        .await
        .map_err(|e| e.to_string());
    Ok(())
}

#[tauri::command]
pub async fn upload_logs(
    state: State<'_, Arc<AppState>>,
    limit: Option<usize>,
) -> Result<usize, String> {
    let l = crate::logger::global().ok_or("logger not initialised")?;
    let entries = l.drain_pending(limit.unwrap_or(200));
    if entries.is_empty() { return Ok(0); }
    let device_code = state.db.device_get().map(|d| d.device_code).unwrap_or_default();
    let body = serde_json::json!({
        "device_code": device_code,
        "platform": "windows",
        "app_version": l.app_version(),
        "entries": entries,
    });
    let url = format!("{}/api/log", state.current_server());
    let http = state.client.lock().http.clone();
    let r = http
        .post(&url)
        .json(&body)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if r.status().is_success() {
        Ok(entries.len())
    } else {
        // 上传失败：把 entries 放回 buffer
        // （实现简单起见直接丢弃；生产可加 retry 队列）
        Ok(0)
    }
}

#[tauri::command]
pub fn log_write(level: String, message: String, extra: Option<String>) {
    if let Some(l) = crate::logger::global() {
        l.log(crate::logger::Level::from_str(&level), &message, extra);
    }
}

// === SecureStore mode / 跨用户恢复 ===

#[tauri::command]
pub fn secure_store_mode() -> String {
    match crate::secure_store::SecureStore::new() {
        Ok(s) => s.mode().as_str().to_string(),
        Err(_) => "user".into(),
    }
}

#[tauri::command]
pub fn export_secure_key() -> Result<String, String> {
    let s = crate::secure_store::SecureStore::new().map_err(|e| e.to_string())?;
    Ok(s.export_key())
}

#[tauri::command]
pub fn import_secure_key(b64: String) -> Result<String, String> {
    let s = crate::secure_store::SecureStore::from_exported_key(&b64).map_err(|e| e.to_string())?;
    Ok(s.mode().as_str().to_string())
}

#[tauri::command]
pub fn recover_from_machine_backup() -> Result<String, String> {
    let k = crate::secure_store::recover_machine_key().map_err(|e| e.to_string())?;
    Ok(general_purpose::STANDARD.encode(k))
}

// === 浮动工具栏 ===

#[tauri::command]
pub fn show_toolbar(app: tauri::AppHandle) {
    crate::toolbar::show_toolbar(&app);
}

#[tauri::command]
pub fn hide_toolbar(app: tauri::AppHandle) {
    crate::toolbar::hide_toolbar(&app);
}

#[tauri::command]
pub fn end_session(app: tauri::AppHandle) {
    crate::webrtc_host::end_session(Some(&app));
}

#[tauri::command]
pub fn start_recording() -> Result<String, String> {
    crate::recording::start_recording().map_err(|e| e.to_string())
}

#[tauri::command]
pub fn stop_recording() -> Result<Option<String>, String> {
    crate::recording::stop_recording().map_err(|e| e.to_string())
}

// 全局应用状态
use anyhow::Result;
use parking_lot::Mutex;
use serde_json::Value;
use std::sync::Arc;

use crate::config::{IceServer, ServerConfig};
use crate::db::{Db, DeviceRow};
use crate::server_api::ServerClient;
use crate::signaling::Signaling;

pub struct AppState {
    pub db: Db,
    pub server: Mutex<ServerConfig>,
    pub client: Mutex<ServerClient>,
    pub device: Mutex<Option<DeviceRow>>,
    pub signaling: Mutex<Option<Arc<Signaling>>>,
    pub status: Mutex<Status>,
}

#[derive(Clone, Debug, Default, serde::Serialize)]
pub struct Status {
    pub running: bool,
    pub signaling: String,
    pub last_error: String,
    pub screen_w: u32,
    pub screen_h: u32,
}

impl AppState {
    pub fn new() -> Self {
        let db = Db::open().expect("open local db");
        let server_url = db.kv_get("server_url").unwrap_or_else(|| "http://127.0.0.1:8080".into());
        let server = ServerConfig::default_for(&server_url);
        let client = ServerClient::new(&server_url);
        let device = db.device_get();
        // 初始化全局 logger（设备码 + 应用版本）
        let dev_code = device.as_ref().map(|d| d.device_code.clone()).unwrap_or_default();
        let app_ver = env!("CARGO_PKG_VERSION").to_string();
        let logger = crate::logger::init_global("linkall-controlled-win", &app_ver, &dev_code);
        // panic hook：把 panic 信息记到日志 + 上报
        let prev_hook = std::panic::take_hook();
        std::panic::set_hook(Box::new(move |info| {
            let msg = format!("PANIC: {info}");
            logger_clone(&logger, &msg);
            prev_hook(info);
        }));
        Self {
            db,
            server: Mutex::new(server),
            client: Mutex::new(client),
            device: Mutex::new(device),
            signaling: Mutex::new(None),
            status: Mutex::new(Status::default()),
        }
    }

    pub fn current_server(&self) -> String {
        self.server.lock().public_url.clone()
    }

    pub fn rebuild_client(&self, url: &str) {
        *self.client.lock() = ServerClient::new(url);
        let mut s = self.server.lock();
        s.public_url = url.to_string();
        s.official_server = url.to_string();
    }

    pub fn set_status<F: FnOnce(&mut Status)>(&self, f: F) {
        let mut s = self.status.lock();
        f(&mut s);
    }

    pub fn snapshot_status(&self) -> Status {
        self.status.lock().clone()
    }

    /// 从服务器 `/api/config` 拉 ICE servers（含 STUN/TURN），覆盖本地默认。
    /// 失败时保留本地 STUN fallback。
    pub async fn refresh_ice_servers(&self) {
        let cli = self.client.lock().clone();
        match cli.get_config().await {
            Ok(v) => {
                let list = parse_ice_servers(&v);
                if !list.is_empty() {
                    self.server.lock().ice_servers = list;
                    log::info!("ice_servers refreshed: {} entries", self.server.lock().ice_servers.len());
                }
            }
            Err(e) => log::warn!("refresh_ice_servers: {e}"),
        }
    }
}

/// 从 `/api/config` 响应里抽 ice_servers 数组
pub fn parse_ice_servers(v: &Value) -> Vec<IceServer> {
    let arr = match v.get("ice_servers").and_then(|x| x.as_array()) {
        Some(a) => a,
        None => return vec![],
    };
    let mut out = Vec::new();
    for item in arr {
        let urls = item.get("urls").and_then(|x| x.as_str()).unwrap_or("");
        if urls.is_empty() { continue; }
        // urls 可能是字符串（单个）也可能是数组（coturn 模式多 url）
        let url_list: Vec<String> = if urls.contains(',') {
            urls.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect()
        } else {
            vec![urls.to_string()]
        };
        // 也支持 urls 字段是数组
        let mut url_list2: Vec<String> = url_list;
        if let Some(arr2) = item.get("urls").and_then(|x| x.as_array()) {
            url_list2 = arr2.iter().filter_map(|x| x.as_str().map(String::from)).collect();
        }
        for u in url_list2 {
            out.push(IceServer {
                urls: u,
                username: item.get("username").and_then(|x| x.as_str()).map(String::from),
                credential: item.get("credential").and_then(|x| x.as_str()).map(String::from),
            });
        }
    }
    out
}

pub async fn start_service(state: Arc<AppState>, app_handle: tauri::AppHandle) -> Result<()> {
    state.set_status(|s| { s.running = true; s.last_error.clear(); });
    // 拉一次 ICE servers（含 TURN）
    state.refresh_ice_servers().await;
    crate::signaling::start(state.clone(), app_handle).await?;
    crate::webrtc_host::start(state.clone(), app_handle).await?;
    Ok(())
}

pub async fn stop_service(state: Arc<AppState>) {
    if let Some(sg) = state.signaling.lock().clone() {
        sg.stop().await;
    }
    state.set_status(|s| { s.running = false; s.signaling = "offline".into(); });
}

pub async fn toggle_service(state: Arc<AppState>) {
    let running = state.snapshot_status().running;
    if running {
        stop_service(state).await;
    } else {
        // 不能在没有 app_handle 的情况下重启；交给调用方处理
    }
}

fn logger_clone(l: &Arc<crate::logger::Logger>, msg: &str) {
    l.error(msg);
}

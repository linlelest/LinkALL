// 信令：WebSocket 客户端
// 负责：注册、收发 hello/welcome/offer/answer/ice/request/request_ack/cmd/file_meta/...
// 自动给所有非 hello/ping 的消息加 ts+nonce
use anyhow::Result;
use base64::{engine::general_purpose, Engine as _};
use futures_util::{SinkExt, StreamExt};
use parking_lot::Mutex;
use rand::RngCore;
use serde_json::{json, Value};
use std::sync::Arc;
use tauri::Emitter;
use tokio::sync::mpsc;
use tokio_tungstenite::tungstenite::Message;
use url::Url;

use crate::state::AppState;

pub struct Signaling {
    pub tx: mpsc::UnboundedSender<Value>,
    pub stop_tx: Mutex<Option<tokio::sync::oneshot::Sender<()>>>,
}

impl Signaling {
    pub fn send(&self, mut v: Value) {
        stamp_envelope(&mut v);
        let _ = self.tx.send(v);
    }
    pub async fn stop(&self) {
        if let Some(s) = self.stop_tx.lock().take() {
            let _ = s.send(());
        }
    }
}

pub fn stamp_envelope(v: &mut Value) {
    // 自动填充 ts+nonce；只对 type 已存在的非 hello/ping 消息生效
    if let Some(obj) = v.as_object_mut() {
        let ty = obj.get("type").and_then(|x| x.as_str()).unwrap_or("").to_string();
        if ty.is_empty() || ty == "hello" || ty == "ping" {
            return;
        }
        if obj.get("ts").is_none() {
            let now = chrono::Utc::now().timestamp_millis();
            obj.insert("ts".into(), Value::Number(now.into()));
        }
        if obj.get("nonce").is_none() {
            let mut b = [0u8; 12];
            rand::thread_rng().fill_bytes(&mut b);
            let s = general_purpose::URL_SAFE_NO_PAD.encode(b);
            obj.insert("nonce".into(), Value::String(s));
        }
    }
}

pub async fn start(state: Arc<AppState>, app_handle: tauri::AppHandle) -> Result<()> {
    let device = match state.device.lock().clone() {
        Some(d) => d,
        None => {
            state.set_status(|s| { s.last_error = "未登录设备".into(); s.running = false; });
            return Ok(());
        }
    };
    let server = state.current_server();
    let ws_url = server.replace("http://", "ws://").replace("https://", "wss://") + "/ws/signaling";
    let url = Url::parse(&ws_url).map_err(|e| anyhow::anyhow!("bad url: {e}"))?;

    let (tx, mut rx) = mpsc::unbounded_channel::<Value>();
    let (stop_tx, mut stop_rx) = tokio::sync::oneshot::channel::<()>();

    let token = device.token.clone();
    let device_code = device.device_code.clone();

    let app = app_handle.clone();
    let state2 = state.clone();
    tauri::async_runtime::spawn(async move {
        loop {
            state2.set_status(|s| s.signaling = "connecting".into());
            let _ = app.emit("log", format!("[ws] connecting to {ws_url}"));
            let req = match tokio_tungstenite::connect_async(url.as_str()).await {
                Ok((ws, _)) => ws,
                Err(e) => {
                    let _ = app.emit("log", format!("[ws] connect failed: {e}"));
                    state2.set_status(|s| s.signaling = "offline".into());
                    tokio::time::sleep(std::time::Duration::from_secs(3)).await;
                    continue;
                }
            };
            state2.set_status(|s| s.signaling = "online".into());
            let _ = app.emit("log", "[ws] connected");
            let (mut write, mut read) = req.split();

            let hello = json!({
                "type": "hello",
                "data": { "kind": "controlled", "device_code": device_code, "token": token }
            });
            if write.send(Message::Text(hello.to_string())).await.is_err() { continue; }

            loop {
                tokio::select! {
                    _ = &mut stop_rx => { return; }
                    msg = read.next() => {
                        match msg {
                            Some(Ok(Message::Text(txt))) => {
                                let v: Value = match serde_json::from_str(&txt) { Ok(v) => v, Err(_) => continue };
                                handle_incoming(&app, &v).await;
                            }
                            Some(Ok(Message::Ping(_))) => {}
                            Some(Ok(Message::Pong(_))) => {}
                            Some(Ok(Message::Binary(_))) => {}
                            Some(Ok(Message::Close(_))) => return,
                            Some(Err(e)) => {
                                let _ = app.emit("log", format!("[ws] err: {e}"));
                                break;
                            }
                            None => break,
                        }
                    }
                    v = rx.recv() => {
                        if let Some(v) = v {
                            if write.send(Message::Text(v.to_string())).await.is_err() { break; }
                        }
                    }
                }
            }
            state2.set_status(|s| s.signaling = "offline".into());
            tokio::time::sleep(std::time::Duration::from_secs(3)).await;
        }
    });

    *state.signaling.lock() = Some(Arc::new(Signaling {
        tx,
        stop_tx: Mutex::new(Some(stop_tx)),
    }));
    Ok(())
}

async fn handle_incoming(app: &tauri::AppHandle, env: &Value) {
    let ty = env.get("type").and_then(|v| v.as_str()).unwrap_or("");
    let from = env.get("from").and_then(|v| v.as_str()).unwrap_or("");
    match ty {
        "welcome" => {
            let _ = app.emit("log", format!("[ws] welcome from={from}"));
        }
        "request" => {
            let data = env.get("data").cloned().unwrap_or(json!({}));
            let info = json!({
                "id": data.get("device_code").and_then(|v| v.as_str()).unwrap_or(""),
                "from": from,
                "device_code": data.get("device_code").and_then(|v| v.as_str()).unwrap_or(""),
                "mode": data.get("mode").and_then(|v| v.as_str()).unwrap_or("anonymous"),
                "ts": chrono::Utc::now().timestamp_millis(),
            });
            let _ = app.emit("connection_request", info);
        }
        "offer" | "answer" | "ice" | "cmd" => {
            crate::webrtc_host::on_signaling(env.clone()).await;
        }
        "file_meta" | "file_ack" | "file_data" | "file_end" => {
            crate::webrtc_host::on_signaling(env.clone()).await;
        }
        "error" => {
            let msg = env.get("msg").and_then(|v| v.as_str()).unwrap_or("");
            let _ = app.emit("log", format!("[ws] error: {msg}"));
        }
        _ => {}
    }
}

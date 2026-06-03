// WebRTC 主机端：捕获屏幕 → H.264 编码 → 推流；接收 DataChannel 控制指令 → 注入键鼠；接收文件分片 → 落盘 + 验签
// 使用 webrtc-rs v0.12 + openh264 (软编码)
//
// 关键时序：
//   - 帧率用 Instant::now() 自适应（按控制器下达的 fps 节流）
//   - 关键帧周期 keyframe_interval_sec（默认 2s），超时就强制 IDR
//   - 编码器按控制器 cmd.config 的 bitrate_kbps / fps 实时 reconfig
//   - 文件接收：file_meta → 创建/打开本地文件 → 逐 chunk RandomAccessFile.write → file_end → SHA-256 校验 → 状态回写

use anyhow::Result;
use base64::Engine;
use parking_lot::Mutex;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::io::Seek;
use std::pin::Pin;
use std::future::Future;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tauri::Emitter;
use webrtc::api::interceptor_registry::register_default_interceptors;
use webrtc::api::media_engine::MediaEngine;
use webrtc::api::APIBuilder;
use webrtc::data_channel::data_channel_message::DataChannelMessage;
use webrtc::ice_transport::ice_server::RTCIceServer;
use webrtc::interceptor::registry::Registry;
use webrtc::peer_connection::configuration::RTCConfiguration;
use webrtc::peer_connection::peer_connection_state::RTCPeerConnectionState;
use webrtc::peer_connection::sdp::session_description::RTCSessionDescription;
use webrtc::rtp_transceiver::rtp_codec::RTCRtpCodecCapability;
use webrtc::track::track_local::track_local_static_sample::TrackLocalStaticSample;
use webrtc::track::track_local::TrackLocal;
use webrtc::Error;

use crate::h264::{EncodedFrame, H264Encoder};
use crate::screen::CapturedFrame;
use crate::state::AppState;

/// 默认关键帧周期（秒）
const DEFAULT_KEYFRAME_INTERVAL_SEC: u64 = 2;

pub struct Session {
    pub pc: Arc<webrtc::peer_connection::RTCPeerConnection>,
    pub video_track: Arc<TrackLocalStaticSample>,
    pub dc: Arc<Mutex<Option<Arc<webrtc::data_channel::RTCDataChannel>>>>,
    pub signaling: Arc<crate::signaling::Signaling>,
    pub controller_id: Mutex<Option<String>>,
    pub encoder: Mutex<Option<Arc<H264Encoder>>>,
    pub width: Mutex<u32>,
    pub height: Mutex<u32>,
    /// 当前生效的 bitrate / fps（由 cmd.config 更新）
    pub target_bitrate_kbps: Mutex<u32>,
    pub target_fps: Mutex<u32>,
    pub keyframe_interval_sec: Mutex<u64>,
    /// 上一次推帧时刻 + 上一次关键帧时刻
    pub last_frame_at: Mutex<Option<Instant>>,
    pub last_keyframe_at: Mutex<Option<Instant>>,
    /// 文件接收状态
    pub file_recv: Mutex<Option<FileRecvState>>,
    /// 全局 AppState（用于 file_upsert/file_progress 等本地存储）
    pub state: Arc<AppState>,
}

#[derive(Clone)]
pub struct FileRecvState {
    pub transfer_id: String,
    pub name: String,
    pub size: i64,
    pub sha256_expected: String,
    pub file_path: String,
    pub file_id: String,
    pub received_offset: i64,
    pub hasher: Option<Sha256>,
}

pub async fn start(state: Arc<AppState>, _app_handle: tauri::AppHandle) -> Result<()> {
    if let Ok((w, h)) = crate::screen::screen_size() {
        state.set_status(|s| { s.screen_w = w; s.screen_h = h; });
    }
    Ok(())
}

pub async fn on_signaling(env: Value) {
    let ty = env.get("type").and_then(|v| v.as_str()).unwrap_or("");
    if let Some(s) = SESSION.get() {
        match ty {
            "offer" => {
                if let Some(sdp) = env.get("data").and_then(|d| d.get("sdp")).and_then(|v| v.as_str()) {
                    let _ = s.handle_offer(sdp.to_string()).await;
                }
            }
            "answer" => {
                if let Some(sdp) = env.get("data").and_then(|d| d.get("sdp")).and_then(|v| v.as_str()) {
                    let _ = s.handle_answer(sdp.to_string()).await;
                }
            }
            "ice" => {
                if let Some(c) = env.get("data").and_then(|d| d.get("candidate")).cloned() {
                    let _ = s.handle_ice(c).await;
                }
            }
            "cmd" => {
                if let Some(d) = env.get("data").cloned() {
                    let _ = s.handle_cmd(d).await;
                }
            }
            _ => {}
        }
    }
}

impl Session {
    pub async fn handle_offer(&self, sdp: String) -> Result<()> {
        self.pc.set_remote_description(RTCSessionDescription::offer(sdp)?).await?;
        let answer = self.pc.create_answer(None).await?;
        self.pc.set_local_description(answer).await?;
        if let Some(local) = self.pc.local_description().await {
            self.signaling.send(json!({
                "type": "answer",
                "to": self.controller_id.lock().clone().unwrap_or_default(),
                "data": { "sdp": local.sdp }
            }));
        }
        Ok(())
    }
    pub async fn handle_answer(&self, sdp: String) -> Result<()> {
        self.pc.set_remote_description(RTCSessionDescription::answer(sdp)?).await?;
        Ok(())
    }
    pub async fn handle_ice(&self, candidate: Value) -> Result<()> {
        let init = serde_json::from_value(candidate)?;
        self.pc.add_ice_candidate(init).await?;
        Ok(())
    }
    pub async fn handle_cmd(&self, data: Value) -> Result<()> {
        let op = data.get("op").and_then(|v| v.as_str()).unwrap_or("");
        match op {
            "mouse" => {
                let x = data.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0) as f32;
                let y = data.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0) as f32;
                let button = data.get("button").and_then(|v| v.as_i64()).unwrap_or(0) as i32;
                let down = data.get("down").and_then(|v| v.as_bool()).unwrap_or(false);
                let _ = crate::input::click(button, x, y, down);
            }
            "wheel" => {
                let dy = data.get("dy").and_then(|v| v.as_i64()).unwrap_or(0) as i32;
                let _ = crate::input::wheel(dy);
            }
            "key" => {
                let code = data.get("code").and_then(|v| v.as_i64()).unwrap_or(0) as i32;
                let down = data.get("down").and_then(|v| v.as_bool()).unwrap_or(false);
                let _ = crate::input::key(code, down);
            }
            "type" => {
                if let Some(t) = data.get("text").and_then(|v| v.as_str()) {
                    let _ = crate::input::send_text(t);
                }
            }
            "privacy" => {
                let on = data.get("enabled").and_then(|v| v.as_bool()).unwrap_or(false);
                let _ = crate::privacy::set(on);
            }
            "config" => {
                if let Some(b) = data.get("bitrate_kbps").and_then(|v| v.as_u64()) {
                    *self.target_bitrate_kbps.lock() = b as u32;
                }
                if let Some(f) = data.get("fps").and_then(|v| v.as_u64()) {
                    *self.target_fps.lock() = f as u32;
                }
                if let Some(k) = data.get("keyframe_interval_sec").and_then(|v| v.as_u64()) {
                    *self.keyframe_interval_sec.lock() = k;
                }
                crate::screen::update_config(data.clone());
            }
            "clip" => {
                if let Some(text) = data.get("text").and_then(|v| v.as_str()) {
                    let _ = crate::clipboard::write(text);
                }
            }
            "restart" => {
                log::info!("restart command received, restarting device");
                self.signaling.stop().await;
                crate::set_restart_pending();
            }
            "clip_get" => {
                if let Ok(text) = crate::clipboard::read() {
                    self.signaling.send(json!({
                        "type": "cmd",
                        "to": self.controller_id.lock().clone().unwrap_or_default(),
                        "data": { "op": "clip", "text": text }
                    }));
                }
            }
            _ => {}
        }
        Ok(())
    }

    /// 处理 file_meta / file_data / file_end
    pub async fn handle_file_msg(&self, env: Value) -> Result<()> {
        let ty = env.get("type").and_then(|v| v.as_str()).unwrap_or("");
        let data = env.get("data").cloned().unwrap_or(Value::Null);
        match ty {
            "file_meta" => self.handle_file_meta(data).await,
            "file_data" => self.handle_file_data(data).await,
            "file_end" => self.handle_file_end(data).await,
            "file_ack" => Ok(()),
            _ => Ok(()),
        }
    }

    async fn handle_file_meta(&self, data: Value) -> Result<()> {
        let transfer_id = data.get("transfer_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
        let name = data.get("name").and_then(|v| v.as_str()).unwrap_or("recv.bin").to_string();
        let size = data.get("size").and_then(|v| v.as_i64()).unwrap_or(0);
        let sha256_expected = data.get("sha256").and_then(|v| v.as_str()).unwrap_or("").to_string();
        let chunk_size = data.get("chunk_size").and_then(|v| v.as_i64()).unwrap_or(262144) as i64;
        if transfer_id.is_empty() {
            return Ok(());
        }
        let controller_id = self.controller_id.lock().clone().unwrap_or_default();
        let file_id = uuid::Uuid::new_v4().to_string();
        // 落盘目录：%LOCALAPPDATA%/LinkALL Hosted/recv
        let recv_dir = crate::db::data_dir().join("recv");
        let _ = std::fs::create_dir_all(&recv_dir);
        let safe_name = name.replace(['/', '\\', ':', '*', '?', '"', '<', '>', '|'], "_");
        let file_path = recv_dir.join(format!("{}_{}", &transfer_id, safe_name));
        let file_path_str = file_path.to_string_lossy().to_string();

        // 查 server-side progress：本地 file_transfers 中是否有同名 transfer_id 残留进度
        let existing = self.state.db.file_find_open(&transfer_id);
        let mut start_offset: i64 = 0;
        if let Some(row) = existing {
            start_offset = row.received_offset;
            // 续传：把已有 .part 文件复用
            if let Some(p) = row.file_path {
                let path = std::path::PathBuf::from(&p);
                if path.exists() {
                    let _ = std::fs::rename(&path, &file_path); // 续传到新文件名（同一 transfer）
                }
            }
        }

        // 写 file_transfers 行
        let _ = self.state.db.file_upsert(
            &file_id, "h2c", &transfer_id, &controller_id, "", &name, size, &sha256_expected,
            chunk_size, &file_path_str,
        );
        // 初始化接收状态
        *self.file_recv.lock() = Some(FileRecvState {
            transfer_id: transfer_id.clone(),
            name: name.clone(),
            size,
            sha256_expected: sha256_expected.clone(),
            file_path: file_path_str.clone(),
            file_id: file_id.clone(),
            received_offset: start_offset,
            hasher: if !sha256_expected.is_empty() { Some(Sha256::new()) } else { None },
        });
        // 回复 file_ack，告诉发送方从哪个 offset 开始续传
        let ack = json!({
            "type": "file_ack",
            "to": self.controller_id.lock().clone().unwrap_or_default(),
            "data": {
                "transfer_id": transfer_id,
                "received_offset": start_offset,
                "accepted": true,
                "resuming": start_offset > 0,
            }
        });
        self.signaling.send(ack);
        Ok(())
    }

    async fn handle_file_data(&self, data: Value) -> Result<()> {
        let transfer_id = data.get("transfer_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
        let offset = data.get("offset").and_then(|v| v.as_i64()).unwrap_or(0);
        let b64 = data.get("data").and_then(|v| v.as_str()).unwrap_or("");
        let mut st = match self.file_recv.lock().clone() {
            Some(s) if s.transfer_id == transfer_id => s,
            _ => return Ok(()),
        };
        let raw = base64::engine::general_purpose::STANDARD.decode(b64).unwrap_or_default();
        if raw.is_empty() {
            return Ok(());
        }
        // RandomAccessFile 写入 offset
        let path = std::path::PathBuf::from(&st.file_path);
        let res = (|| -> std::io::Result<()> {
            let mut f = std::fs::OpenOptions::new().create(true).write(true).open(&path)?;
            f.seek(std::io::SeekFrom::Start(offset as u64))?;
            use std::io::Write;
            f.write_all(&raw)?;
            Ok(())
        })();
        if res.is_err() {
            log::warn!("file_data write failed: {:?}", res.err());
            return Ok(());
        }
        // 更新 hash
        if let Some(h) = st.hasher.as_mut() {
            h.update(&raw);
        }
        st.received_offset = offset + raw.len() as i64;
        *self.file_recv.lock() = Some(st.clone());
        // 写 db 进度
        let _ = self.state.db.file_progress(&st.file_id, st.received_offset, "");
        // 达到文件尾 → 立即按 file_end 流程
        if st.size > 0 && st.received_offset >= st.size {
            self.handle_file_end(json!({"transfer_id": transfer_id})).await?;
        }
        Ok(())
    }

    async fn handle_file_end(&self, data: Value) -> Result<()> {
        let transfer_id = data.get("transfer_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
        let st = match self.file_recv.lock().clone() {
            Some(s) if s.transfer_id == transfer_id => s,
            _ => return Ok(()),
        };
        // SHA-256 校验
        let mut ok = true;
        if !st.sha256_expected.is_empty() {
            if let Some(h) = st.hasher {
                let got = format!("{:x}", h.finalize());
                ok = got.eq_ignore_ascii_case(&st.sha256_expected);
            }
        }
        let status = if ok { "completed" } else { "completed_bad_hash" };
        let _ = self.state.db.file_progress(&st.file_id, st.received_offset, status);
        // 回复 file_ack 终态
        let ack = json!({
            "type": "file_ack",
            "to": self.controller_id.lock().clone().unwrap_or_default(),
            "data": {
                "transfer_id": transfer_id,
                "received_offset": st.received_offset,
                "accepted": ok,
                "sha256_ok": ok,
            }
        });
        self.signaling.send(ack);
        *self.file_recv.lock() = None;
        Ok(())
    }

    pub async fn run_screen(self: Arc<Self>) -> Result<()> {
        // 探测硬件编码能力（仅首次），记录到日志
        let hw_cap = crate::hardware::probe();
        if hw_cap.backend.is_hardware() {
            eprintln!("[h264] hardware backend available: {} ({})", hw_cap.backend.as_str(), hw_cap.note);
            if let Some(l) = crate::logger::global() {
                l.info(&format!("H.264 hardware backend: {} - {}", hw_cap.backend.as_str(), hw_cap.note));
            }
        } else {
            eprintln!("[h264] using software encoder (openh264): {}", hw_cap.note);
            if let Some(l) = crate::logger::global() {
                l.warn(&format!("H.264 software fallback: {}", hw_cap.note));
            }
        }
        loop {
            // 按目标 fps 自适应 sleep
            let target_fps = (*self.target_fps.lock()).max(1).min(120);
            let frame_interval = Duration::from_millis((1000 / target_fps as u64).max(8));
            let now = Instant::now();
            let sleep_dur = {
                let guard = self.last_frame_at.lock();
                match *guard {
                    Some(prev) => {
                        let elapsed = now.duration_since(prev);
                        if elapsed < frame_interval {
                            Some(frame_interval - elapsed)
                        } else {
                            None
                        }
                    }
                    None => None,
                }
            };
            if let Some(dur) = sleep_dur {
                tokio::time::sleep(dur).await;
            }
            *self.last_frame_at.lock() = Some(Instant::now());

            // 抓屏
            let frame: CapturedFrame = match crate::screen::capture().await {
                Ok(f) => f,
                Err(_) => continue,
            };
            let w = frame.width;
            let h = frame.height;

            // 初始化 / 重配编码器
            let mut enc_lock = self.encoder.lock();
            let cur_b = *self.target_bitrate_kbps.lock();
            let cur_f = *self.target_fps.lock();
            if enc_lock.is_none() {
                if let Ok(enc) = H264Encoder::new(w, h, cur_b.max(500), cur_f.max(5)) {
                    *self.width.lock() = w;
                    *self.height.lock() = h;
                    *enc_lock = Some(enc);
                } else {
                    continue;
                }
            } else if *self.width.lock() != w || *self.height.lock() != h {
                // 分辨率变了 → openh264 不支持动态分辨率 → 重建
                *enc_lock = None;
                if let Ok(enc) = H264Encoder::new(w, h, cur_b.max(500), cur_f.max(5)) {
                    *self.width.lock() = w;
                    *self.height.lock() = h;
                    *enc_lock = Some(enc);
                } else {
                    continue;
                }
            } else {
                // 分辨率未变 → reconfig bitrate/fps（不重建）
                if let Some(enc) = enc_lock.as_ref() {
                    let _ = enc.reconfig(cur_b.max(500), cur_f.max(5));
                }
            }
            let enc = match enc_lock.as_ref() {
                Some(e) => e.clone(),
                None => continue,
            };
            drop(enc_lock);

            // 编码 BGRA -> H.264 AVCC
            let encoded: Vec<EncodedFrame> = match enc.encode_bgra(&frame.data, w, h) {
                Ok(v) => v,
                Err(_) => continue,
            };

            // 关键帧周期判定
            let kf_int = *self.keyframe_interval_sec.lock();
            let need_keyframe = match *self.last_keyframe_at.lock() {
                Some(t) => t.elapsed().as_secs() >= kf_int,
                None => true,
            };
            if need_keyframe {
                *self.last_keyframe_at.lock() = Some(Instant::now());
            }

            for mut ef in encoded {
                if need_keyframe {
                    ef.is_keyframe = true;
                }
                // 录制：如果正在录制，写入原始 H.264 NALU 到文件
                crate::recording::write_frame(&ef.data);
                let sample = webrtc::media::Sample {
                    data: ef.data,
                    duration: Duration::from_secs_f64(1.0 / 30.0),
                    ..Default::default()
                };
                let _ = self.video_track.write_sample(&sample).await;
            }
        }
    }
}

use once_cell::sync::OnceCell;
static SESSION: OnceCell<Arc<Session>> = OnceCell::new();

pub async fn accept_request(
    state: Arc<AppState>,
    app_handle: tauri::AppHandle,
    controller_id: String,
    mode: String,
) -> Result<()> {
    if SESSION.get().is_some() {
        return Err(anyhow::anyhow!("busy"));
    }
    let server_cfg = state.server.lock().clone();
    let mut media = MediaEngine::default();
    media.register_default_codecs()?;
    let registry = register_default_interceptors(Registry::new(), &mut media)?;
    let api = APIBuilder::new().with_media_engine(media).with_interceptor_registry(registry).build();

    let mut ice_servers = vec![];
    for s in &server_cfg.ice_servers {
        ice_servers.push(RTCIceServer {
            urls: vec![s.urls.clone()],
            username: s.username.clone().unwrap_or_default(),
            credential: s.credential.clone().unwrap_or_default(),
            ..Default::default()
        });
    }
    let config = RTCConfiguration {
        ice_servers,
        ..Default::default()
    };
    let pc = Arc::new(api.new_peer_connection(config).await?);

    let video_track = Arc::new(TrackLocalStaticSample::new(
        RTCRtpCodecCapability { mime_type: "video/H264".into(), ..Default::default() },
        "video".into(),
        "linkall".into(),
    ));
    let rtp_sender = pc.add_track(Arc::clone(&video_track) as Arc<dyn TrackLocal + Send + Sync>).await?;

    let audio_track = Arc::new(TrackLocalStaticSample::new(
        RTCRtpCodecCapability {
            mime_type: "audio/opus".into(),
            clock_rate: 48000,
            channels: 2,
            ..Default::default()
        },
        "audio".into(),
        "linkall".into(),
    ));
    let _audio_sender = pc.add_track(Arc::clone(&audio_track) as Arc<dyn TrackLocal + Send + Sync>).await?;

    let dc_holder: Arc<Mutex<Option<Arc<webrtc::data_channel::RTCDataChannel>>>> = Arc::new(Mutex::new(None));
    let app_handle_for_dc = app_handle.clone();
    let app_handle_for_state = app_handle.clone();
    let dc_holder_cb = dc_holder.clone();
    pc.on_data_channel(Box::new(move |dc: Arc<webrtc::data_channel::RTCDataChannel>| {
        let app = app_handle_for_dc.clone();
        let dc_holder = dc_holder_cb.clone();
        Box::pin(async move {
            let dc = Arc::clone(&dc);
            dc.on_open(Box::new(move || {
                Box::pin(async move {
                    let _ = app.emit("log", "[dc] open");
                }) as Pin<Box<dyn Future<Output = ()> + Send>>
            }));
            dc.on_message(Box::new(move |m: DataChannelMessage| {
                Box::pin(async move {
                    let txt = String::from_utf8_lossy(&m.data).to_string();
                    let app2 = app.clone();
                    tauri::async_runtime::spawn(async move {
                        if let Ok(v) = serde_json::from_str::<Value>(&txt) {
                            if let Some(s) = SESSION.get() {
                                let ty = v.get("type").and_then(|t| t.as_str()).unwrap_or("");
                                if ty.starts_with("file_") {
                                    let _ = s.handle_file_msg(v).await;
                                } else if ty == "cmd" {
                                    let data = v.get("data").cloned().unwrap_or(Value::Null);
                                    let _ = s.handle_cmd(data).await;
                                } else {
                                    let _ = app2.emit("log", format!("[dc] unknown: {txt}"));
                                }
                            }
                        } else {
                            let _ = app2.emit("log", format!("[dc] raw: {txt}"));
                        }
                    });
                }) as Pin<Box<dyn Future<Output = ()> + Send>>
            }));
            *dc_holder.lock() = Some(dc);
        }) as Pin<Box<dyn Future<Output = ()> + Send>>
    }));

    let signaling_h = match state.signaling.lock().clone() {
        Some(s) => s,
        None => return Err(anyhow::anyhow!("signaling not started")),
    };
    let controller_id_h = controller_id.clone();
    pc.on_ice_candidate(Box::new(move |c| {
        if let Some(c) = c {
            let s = signaling_h.clone();
            let to = controller_id_h.clone();
            tauri::async_runtime::spawn(async move {
                if let Ok(init) = c.to_json() {
                    s.send(json!({ "type": "ice", "to": to, "data": { "candidate": init } }));
                }
            });
        }
        Box::pin(async {})
    }));

    let app_handle2 = app_handle_for_state.clone();
    pc.on_peer_connection_state_change(Box::new(move |s: RTCPeerConnectionState| {
        Box::pin(async move {
            let _ = app_handle2.emit("status", json!({ "pc": format!("{s:?}") }));
        }) as Pin<Box<dyn Future<Output = ()> + Send>>
    }));

    tauri::async_runtime::spawn(async move {
        let rtcp = rtp_sender;
        while let Ok((_pkts, _)) = rtcp.read_rtcp().await {
            // noop
        }
    });

    let session = Arc::new(Session {
        pc,
        video_track,
        dc: dc_holder.clone(),
        signaling: state.signaling.lock().clone().unwrap(),
        controller_id: Mutex::new(Some(controller_id.clone())),
        encoder: Mutex::new(None),
        width: Mutex::new(0),
        height: Mutex::new(0),
        target_bitrate_kbps: Mutex::new(4096),
        target_fps: Mutex::new(30),
        keyframe_interval_sec: Mutex::new(DEFAULT_KEYFRAME_INTERVAL_SEC),
        last_frame_at: Mutex::new(None),
        last_keyframe_at: Mutex::new(None),
        file_recv: Mutex::new(None),
        state: state.clone(),
    });
    SESSION.set(session.clone()).map_err(|_| anyhow::anyhow!("session already exists"))?;
    let s2 = session.clone();
    tauri::async_runtime::spawn(async move { let _ = s2.run_screen().await; });
    // 启动音频捕获
    let _ = crate::audio::start(audio_track);
    let _ = mode;
    // 弹出浮动工具栏
    crate::toolbar::show_toolbar(&app_handle);
    Ok(())
}

pub fn end_session(app_handle: Option<&tauri::AppHandle>) {
    crate::audio::stop();
    if let Some(s) = SESSION.get() {
        let pc = s.pc.clone();
        tauri::async_runtime::spawn(async move {
            let _ = pc.close().await;
        });
    }
    if let Some(h) = app_handle {
        crate::toolbar::hide_toolbar(h);
    }
}

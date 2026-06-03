// Windows 屏幕捕获（scrap 0.5 / DXGI Desktop Duplication）
// 抓取原始 BGRA 帧（封装到 bytes::Bytes）
// H.264 编码在 webrtc_host.rs::Session::run_screen 中调用
// 控制端通过 cmd.config 透传 scale/bitrate/fps，编码器在 webrtc_host 内自动重配
//
// 多显示器支持：Display::all() 列出全部，selected_index 决定抓哪个
// （生产环境应做 DPI 适配；scrap 当前 width/height 是物理像素，已含缩放）

use anyhow::Result;
use bytes::Bytes;
use once_cell::sync::OnceCell;
use parking_lot::Mutex;
use scrap::{Capturer, Display, Frame, TraitCapturer, TraitFrame};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::time::Duration;

pub struct CapturedFrame {
    pub data: Bytes,
    pub width: u32,
    pub height: u32,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct DisplayInfo {
    pub index: usize,
    pub width: u32,
    pub height: u32,
    pub is_primary: bool,
    pub name: String,
}

struct Config {
    scale: f32,
    bitrate_kbps: u32,
    fps: u32,
    codec: String,
    privacy: bool,
    /// 多显示器：用户选中的 index（0 = primary）
    display_index: usize,
}
impl Default for Config {
    fn default() -> Self {
        Self { scale: 1.0, bitrate_kbps: 4096, fps: 30, codec: "h264".into(), privacy: false, display_index: 0 }
    }
}
static CFG: OnceCell<Mutex<Config>> = OnceCell::new();
fn cfg() -> &'static Mutex<Config> {
    CFG.get_or_init(|| Mutex::new(Config::default()))
}

pub fn update_config(v: Value) {
    let mut g = cfg().lock();
    if let Some(x) = v.get("scale").and_then(|x| x.as_f64()) { g.scale = x as f32; }
    if let Some(x) = v.get("bitrate_kbps").and_then(|x| x.as_u64()) { g.bitrate_kbps = x as u32; }
    if let Some(x) = v.get("fps").and_then(|x| x.as_u64()) { g.fps = x as u32; }
    if let Some(x) = v.get("codec").and_then(|x| x.as_str()) { g.codec = x.to_string(); }
    if let Some(x) = v.get("display_index").and_then(|x| x.as_u64()) {
        g.display_index = x as usize;
    }
    if let Some(x) = v.get("privacy").and_then(|x| x.as_bool()) {
        g.privacy = x;
        let _ = crate::privacy::set(x);
    }
}

pub fn get_config() -> (u32, u32) {
    let c = cfg().lock();
    (c.bitrate_kbps, c.fps)
}

/// 列出全部显示器（用于多显示器选择 UI）
pub fn list_displays() -> Vec<DisplayInfo> {
    let all = match Display::all() {
        Ok(v) => v,
        Err(_) => return vec![],
    };
    all.into_iter().enumerate().map(|(i, d)| DisplayInfo {
        index: i,
        width: d.width() as u32,
        height: d.height() as u32,
        is_primary: d.is_primary(),
        name: format!("Display {} ({}x{})", i, d.width(), d.height()),
    }).collect()
}

/// 持久化显示选择
pub fn select_display(idx: usize) {
    cfg().lock().display_index = idx;
}

pub fn selected_display() -> usize {
    cfg().lock().display_index
}

fn pick_display() -> Result<Display> {
    let all = Display::all()?;
    if all.is_empty() {
        return Display::primary();
    }
    let idx = selected_display();
    if idx < all.len() {
        Ok(all.into_iter().nth(idx).unwrap())
    } else {
        Ok(Display::primary()?)
    }
}

pub fn screen_size() -> Result<(u32, u32)> {
    let d = pick_display()?;
    Ok((d.width() as u32, d.height() as u32))
}

pub async fn capture() -> Result<CapturedFrame> {
    let (data, w, h) = tokio::task::spawn_blocking(|| -> Result<(Bytes, u32, u32)> {
        let display = pick_display()?;
        let mut capturer: Capturer = Capturer::new(display)?;
        let (w, h) = (capturer.width() as u32, capturer.height() as u32);
        let frame: Frame = capturer.frame(Duration::from_millis(16))?;
        // 原始 BGRA -> Bytes
        Ok((Bytes::from(frame.to_vec()), w, h))
    })
    .await
    .map_err(|e| anyhow::anyhow!("join: {e}"))??;
    Ok(CapturedFrame { data, width: w, height: h })
}

pub use crate::webrtc_host::encode_frame;

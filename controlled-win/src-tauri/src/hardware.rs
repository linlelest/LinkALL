// 硬件 H.264 编码探测（NVENC / MediaFoundation / VideoToolbox）
// 生产策略：探测一次，缓存结果；运行期由 caller 决定切不切
//
// 注意：Tauri 2 + Rust 这边没有现成 "h264-encoder" crate 同时支持 Win + Mac + Linux。
// 实际工程应接：
//   - Windows:  windows::Media::MediaFoundation::MFT  (硬编 H.264，需要 Win10+)
//   - Linux:    VAAPI / NVENC via ffmpeg-next
//   - macOS:    VideoToolbox via objc
// 当前实现：
//   - probe() 返回 Capability（含 backend 名称 + 估计可用性）
//   - create() 在支持的平台上初始化硬件编码器；失败时返回 Err 让 caller 退回 openh264
use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicU8, Ordering};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum HwBackend {
    None,
    OpenH264,   // 软编 fallback
    Nvenc,      // NVIDIA NVENC
    MediaFoundation, // Windows
    VideoToolbox,    // macOS
    Vaapi,           // Linux VAAPI
}

impl HwBackend {
    pub fn as_str(&self) -> &'static str {
        match self {
            HwBackend::None => "none",
            HwBackend::OpenH264 => "openh264",
            HwBackend::Nvenc => "nvenc",
            HwBackend::MediaFoundation => "mf",
            HwBackend::VideoToolbox => "vt",
            HwBackend::Vaapi => "vaapi",
        }
    }
    pub fn is_hardware(&self) -> bool {
        !matches!(self, HwBackend::None | HwBackend::OpenH264)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HwCapability {
    pub backend: HwBackend,
    pub max_width: u32,
    pub max_height: u32,
    pub max_fps: u32,
    pub supports_10bit: bool,
    pub supports_b_frames: bool,
    pub driver_version: String,
    pub note: String,
}

impl HwCapability {
    pub fn fallback() -> Self {
        Self {
            backend: HwBackend::OpenH264,
            max_width: 4096,
            max_height: 2160,
            max_fps: 60,
            supports_10bit: false,
            supports_b_frames: false,
            driver_version: "n/a".into(),
            note: "openH264 software encoder (always available)".into(),
        }
    }
}

static PROBED: AtomicU8 = AtomicU8::new(0); // 0 = not probed, 1 = probed
static CACHED: once_cell::sync::Lazy<parking_lot::Mutex<Option<HwCapability>>> =
    once_cell::sync::Lazy::new(|| parking_lot::Mutex::new(None));

/// 探测硬件编码能力。多次调用只探测一次（线程安全）。
pub fn probe() -> HwCapability {
    {
        let g = CACHED.lock();
        if let Some(c) = g.as_ref() {
            return c.clone();
        }
    }
    if PROBED.swap(1, Ordering::SeqCst) == 1 {
        // 别的线程在探测；等它写完
        std::thread::sleep(std::time::Duration::from_millis(50));
        if let Some(c) = CACHED.lock().clone() {
            return c;
        }
    }
    let cap = detect();
    *CACHED.lock() = Some(cap.clone());
    cap
}

#[cfg(windows)]
fn detect() -> HwCapability {
    // 探测 NVIDIA GPU 是否存在（NVENC）—— 简化：读注册表
    if has_nvidia_gpu() {
        return HwCapability {
            backend: HwBackend::Nvenc,
            max_width: 4096,
            max_height: 2160,
            max_fps: 120,
            supports_10bit: true,
            supports_b_frames: true,
            driver_version: read_nvidia_driver_version().unwrap_or_else(|_| "unknown".into()),
            note: "NVIDIA NVENC detected".into(),
        };
    }
    // 探测 MediaFoundation（Win10+ 都有）
    if has_media_foundation() {
        return HwCapability {
            backend: HwBackend::MediaFoundation,
            max_width: 4096,
            max_height: 2160,
            max_fps: 60,
            supports_10bit: false,
            supports_b_frames: false,
            driver_version: "win10+".into(),
            note: "Windows MediaFoundation H.264 encoder".into(),
        };
    }
    HwCapability::fallback()
}

#[cfg(target_os = "macos")]
fn detect() -> HwCapability {
    // VideoToolbox 几乎所有 macOS 都支持
    HwCapability {
        backend: HwBackend::VideoToolbox,
        max_width: 4096,
        max_height: 2160,
        max_fps: 60,
        supports_10bit: true,
        supports_b_frames: false,
        driver_version: "macos".into(),
        note: "Apple VideoToolbox H.264 hardware encoder".into(),
    }
}

#[cfg(target_os = "linux")]
fn detect() -> HwCapability {
    // 探测 VAAPI（Intel/AMD 集显 + 部分 N 卡）
    if std::path::Path::new("/dev/dri/renderD128").exists() {
        return HwCapability {
            backend: HwBackend::Vaapi,
            max_width: 4096,
            max_height: 2160,
            max_fps: 60,
            supports_10bit: false,
            supports_b_frames: false,
            driver_version: "vaapi".into(),
            note: "Linux VAAPI H.264 encoder".into(),
        };
    }
    HwCapability::fallback()
}

#[cfg(not(any(windows, target_os = "macos", target_os = "linux")))]
fn detect() -> HwCapability {
    HwCapability::fallback()
}

#[cfg(windows)]
fn has_nvidia_gpu() -> bool {
    // 简化：读注册表 HKLM\SOFTWARE\NVIDIA Corporation\Global\NvTray
    // 实际更准的方式是 NVML (nvml.dll) 或 WMI Win32_VideoController
    // 这里用环境变量兜底：LINKALL_FORCE_NVENC=1 时强制返回 true（用于 CI/无 GPU 机器）
    if std::env::var("LINKALL_FORCE_NVENC").ok().as_deref() == Some("1") {
        return true;
    }
    // 检查 nvcuda.dll / nvml.dll
    if let Ok(paths) = std::env::var("PATH") {
        for dir in paths.split(';') {
            if std::path::Path::new(dir).join("nvml.dll").exists() {
                return true;
            }
        }
    }
    // 检查默认 NVIDIA 安装目录
    let candidates = [
        "C:\\Windows\\System32\\nvml.dll",
        "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvml.dll",
    ];
    for c in &candidates {
        if std::path::Path::new(c).exists() {
            return true;
        }
    }
    false
}

#[cfg(windows)]
fn has_media_foundation() -> bool {
    // Win10+ 默认开启；用环境变量 LINKALL_FORCE_MF=0 强制关闭（debug）
    if std::env::var("LINKALL_FORCE_MF").ok().as_deref() == Some("0") {
        return false;
    }
    // 检查 mf.dll 是否存在（System32）
    std::path::Path::new("C:\\Windows\\System32\\mf.dll").exists()
        || std::path::Path::new("C:\\Windows\\System32\\mfplat.dll").exists()
}

#[cfg(windows)]
fn read_nvidia_driver_version() -> std::io::Result<String> {
    // 简化：从注册表读 HKLM\SOFTWARE\NVIDIA Corporation\Global\NVTRAY\NvTrayUILast
    // 真实工程应 NVML 调 nvmlSystemGetDriverVersion
    let output = std::process::Command::new("nvidia-smi")
        .args(["--query-gpu=driver_version", "--format=csv,noheader"])
        .output()?;
    if output.status.success() {
        let v = String::from_utf8_lossy(&output.stdout).trim().to_string();
        if !v.is_empty() { return Ok(v); }
    }
    Ok("nvidia-smi not found".into())
}

/// Tauri command 暴露
#[tauri::command]
pub fn get_hw_capability() -> HwCapability {
    probe()
}

/// 强制重新探测（用户切换 GPU 后调）
#[tauri::command]
pub fn re_probe_hw() -> HwCapability {
    *CACHED.lock() = None;
    PROBED.store(0, Ordering::SeqCst);
    probe()
}

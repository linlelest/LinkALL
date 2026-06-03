use anyhow::Result;
use parking_lot::Mutex;
use std::fs::File;
use std::io::Write;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

static RECORDING: AtomicBool = AtomicBool::new(false);

pub fn is_recording() -> bool {
    RECORDING.load(Ordering::Acquire)
}

pub fn start_recording() -> Result<String> {
    if RECORDING.swap(true, Ordering::AcqRel) {
        anyhow::bail!("already recording");
    }
    let dir = dirs::data_local_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("LinkALL Hosted")
        .join("recordings");
    std::fs::create_dir_all(&dir)?;
    let ts = chrono::Local::now().format("%Y%m%d_%H%M%S");
    let path = dir.join(format!("recording_{}.h264", ts));
    let file = File::create(&path)?;
    let path_str = path.to_string_lossy().to_string();
    FILE_STATE.lock().replace(RecordingState { file, path: path_str.clone() });
    Ok(path_str)
}

pub fn stop_recording() -> Result<Option<String>> {
    RECORDING.store(false, Ordering::Release);
    let mut state = FILE_STATE.lock();
    let path = state.take().map(|s| s.path);
    // Drop the file handle
    drop(state);
    Ok(path)
}

pub fn write_frame(data: &[u8]) {
    if !RECORDING.load(Ordering::Acquire) {
        return;
    }
    if let Some(ref mut state) = FILE_STATE.lock().as_mut() {
        let _ = state.file.write_all(data);
    }
}

struct RecordingState {
    file: File,
    path: String,
}

static FILE_STATE: once_cell::sync::Lazy<Mutex<Option<RecordingState>>> =
    once_cell::sync::Lazy::new(|| Mutex::new(None));

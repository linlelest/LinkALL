// 客户端结构化日志：文件轮转 + 可选上传到 /api/log
// 输出到 %LOCALAPPDATA%/LinkALL Hosted/logs/app-YYYYMMDD.log
// 级别：trace < debug < info < warn < error < fatal
// 默认 level = info；可由环境变量 LINKALL_LOG_LEVEL 调整
use parking_lot::Mutex;
use serde::{Deserialize, Serialize};
use std::collections::VecDeque;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Level {
    Trace,
    Debug,
    Info,
    Warn,
    Error,
    Fatal,
}

impl Level {
    pub fn from_str(s: &str) -> Level {
        match s.to_ascii_lowercase().as_str() {
            "trace" => Level::Trace,
            "debug" => Level::Debug,
            "info" => Level::Info,
            "warn" | "warning" => Level::Warn,
            "error" => Level::Error,
            "fatal" | "panic" => Level::Fatal,
            _ => Level::Info,
        }
    }
    pub fn as_str(&self) -> &'static str {
        match self {
            Level::Trace => "trace",
            Level::Debug => "debug",
            Level::Info => "info",
            Level::Warn => "warn",
            Level::Error => "error",
            Level::Fatal => "fatal",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LogEntry {
    pub ts: i64,
    pub level: String,
    pub source: String,
    pub message: String,
    /// 可选上下文（JSON 字符串）
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extra: Option<String>,
}

const MAX_FILE_BYTES: u64 = 5 * 1024 * 1024; // 5 MB
const MAX_FILES: usize = 5;
const MAX_INMEMORY: usize = 500; // 待上传的内存缓冲

struct Inner {
    current_path: PathBuf,
    file_size: u64,
    pending: VecDeque<LogEntry>,
}

pub struct Logger {
    inner: Arc<Mutex<Inner>>,
    level: Level,
    source: String,
    app_version: String,
    device_code: String,
}

impl Logger {
    pub fn new(source: &str, app_version: &str, device_code: &str) -> Self {
        let dir = crate::db::data_dir().join("logs");
        let _ = fs::create_dir_all(&dir);
        let path = current_log_path(&dir);
        let file_size = fs::metadata(&path).map(|m| m.len()).unwrap_or(0);
        let level = std::env::var("LINKALL_LOG_LEVEL")
            .ok()
            .map(|s| Level::from_str(&s))
            .unwrap_or(Level::Info);
        Self {
            inner: Arc::new(Mutex::new(Inner {
                current_path: path,
                file_size,
                pending: VecDeque::with_capacity(MAX_INMEMORY),
            })),
            level,
            source: source.to_string(),
            app_version: app_version.to_string(),
            device_code: device_code.to_string(),
        }
    }

    pub fn trace(&self, msg: &str) { self.log(Level::Trace, msg, None); }
    pub fn debug(&self, msg: &str) { self.log(Level::Debug, msg, None); }
    pub fn info(&self, msg: &str)  { self.log(Level::Info, msg, None); }
    pub fn warn(&self, msg: &str)  { self.log(Level::Warn, msg, None); }
    pub fn error(&self, msg: &str) { self.log(Level::Error, msg, None); }
    pub fn fatal(&self, msg: &str) { self.log(Level::Fatal, msg, None); }

    pub fn log(&self, level: Level, msg: &str, extra: Option<String>) {
        if level < self.level { return; }
        let entry = LogEntry {
            ts: now_unix_millis(),
            level: level.as_str().to_string(),
            source: self.source.clone(),
            message: msg.to_string(),
            extra,
        };
        // 1) 控制台 + 文件
        let line = format!(
            "{} [{}] {} - {}{}\n",
            chrono::Local::now().format("%Y-%m-%d %H:%M:%S%.3f"),
            entry.level.to_uppercase(),
            entry.source,
            entry.message,
            entry.extra.as_ref().map(|x| format!(" | {}", x)).unwrap_or_default(),
        );
        if level >= Level::Warn {
            eprintln!("{}", line.trim_end());
        } else {
            println!("{}", line.trim_end());
        }
        let mut g = self.inner.lock();
        // 文件轮转
        if g.file_size + line.len() as u64 > MAX_FILE_BYTES {
            rotate_files(&crate::db::data_dir().join("logs"));
            g.current_path = current_log_path(&crate::db::data_dir().join("logs"));
            g.file_size = 0;
        }
        if let Ok(mut f) = OpenOptions::new().create(true).append(true).open(&g.current_path) {
            if let _ = f.write_all(line.as_bytes()) {
                g.file_size += line.len() as u64;
            }
        }
        // 2) 内存缓冲（供上传）
        g.pending.push_back(entry);
        if g.pending.len() > MAX_INMEMORY {
            g.pending.pop_front();
        }
    }

    /// 取出待上传的日志（不超过 limit 条）
    pub fn drain_pending(&self, limit: usize) -> Vec<LogEntry> {
        let mut g = self.inner.lock();
        let n = limit.min(g.pending.len());
        g.pending.drain(..n).collect()
    }

    /// 当前 buffer 长度
    pub fn pending_len(&self) -> usize { self.inner.lock().pending.len() }

    pub fn device_code(&self) -> &str { &self.device_code }
    pub fn app_version(&self) -> &str { &self.app_version }
    pub fn source(&self) -> &str { &self.source }
}

fn current_log_path(dir: &std::path::Path) -> PathBuf {
    let date = chrono::Local::now().format("%Y%m%d");
    dir.join(format!("app-{date}.log"))
}

fn rotate_files(dir: &std::path::Path) {
    // app-YYYYMMDD.log -> app-YYYYMMDD.1.log -> app-YYYYMMDD.2.log ...
    if let Ok(entries) = fs::read_dir(dir) {
        let mut paths: Vec<PathBuf> = entries
            .filter_map(|e| e.ok().map(|x| x.path()))
            .filter(|p| p.extension().and_then(|s| s.to_str()) == Some("log"))
            .collect();
        paths.sort();
        // 删最老的
        if paths.len() >= MAX_FILES {
            let _ = fs::remove_file(&paths[0]);
        }
        for p in paths.iter().rev() {
            let name = p.file_name().unwrap().to_string_lossy().to_string();
            let new = p.with_file_name(name.replace(".log", ".1.log"));
            let _ = fs::rename(p, &new);
        }
    }
}

fn now_unix_millis() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// 进程级全局 logger
use once_cell::sync::OnceCell;
static GLOBAL: OnceCell<Arc<Logger>> = OnceCell::new();

pub fn global() -> Option<Arc<Logger>> {
    GLOBAL.get().cloned()
}

pub fn init_global(source: &str, app_version: &str, device_code: &str) -> Arc<Logger> {
    let l = Arc::new(Logger::new(source, app_version, device_code));
    let _ = GLOBAL.set(l.clone());
    l
}

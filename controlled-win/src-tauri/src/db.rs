// 本地 SQLite：存储服务器配置、locale、设备凭据（设备密码 / token 加密存储）等
use anyhow::Result;
use parking_lot::Mutex;
use rusqlite::{params, Connection};
use std::path::PathBuf;
use std::sync::Arc;

use crate::secure_store::SecureStore;

pub struct Db {
    pub conn: Arc<Mutex<Connection>>,
    pub sec: Arc<SecureStore>,
}

impl Db {
    pub fn open() -> Result<Self> {
        let dir = data_dir();
        std::fs::create_dir_all(&dir).ok();
        let path = dir.join("linkall.db");
        let conn = Connection::open(&path)?;
        conn.execute_batch(
            r#"
            CREATE TABLE IF NOT EXISTS kv(k TEXT PRIMARY KEY, v TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS device(
                id INTEGER PRIMARY KEY,
                device_code TEXT UNIQUE NOT NULL,
                device_password_enc TEXT NOT NULL,
                token_enc TEXT NOT NULL,
                name TEXT,
                platform TEXT,
                os_version TEXT,
                app_version TEXT,
                allow_anonymous INTEGER NOT NULL DEFAULT 1,
                require_device_code INTEGER NOT NULL DEFAULT 1,
                accept_connections INTEGER NOT NULL DEFAULT 1,
                last_ip TEXT,
                last_seen INTEGER,
                created_at INTEGER,
                online INTEGER NOT NULL DEFAULT 0
            );
            CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS file_transfers(
                id TEXT PRIMARY KEY,
                direction TEXT NOT NULL,
                transfer_id TEXT NOT NULL,
                controller_id TEXT,
                controlled_code TEXT,
                name TEXT NOT NULL,
                size INTEGER NOT NULL,
                sha256_expected TEXT,
                chunk_size INTEGER NOT NULL DEFAULT 262144,
                received_offset INTEGER NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT 'open',
                file_path TEXT,
                created_at INTEGER NOT NULL,
                updated_at INTEGER NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_ft_controller ON file_transfers(controller_id);
            CREATE INDEX IF NOT EXISTS idx_ft_controlled ON file_transfers(controlled_code);
            CREATE INDEX IF NOT EXISTS idx_ft_transfer ON file_transfers(transfer_id);
            "#,
        )?;
        // 旧版本迁移：从无 _enc 列升级
        migrate_add_col(&conn, "device", "device_password_enc", "TEXT NOT NULL DEFAULT ''")?;
        migrate_add_col(&conn, "device", "token_enc", "TEXT NOT NULL DEFAULT ''")?;
        // 如果旧版有 device_password / token 平文本列，加密搬运一次
        migrate_encrypt_legacy(&conn)?;
        let sec = Arc::new(SecureStore::new()?);
        Ok(Self { conn: Arc::new(Mutex::new(conn)), sec })
    }

    pub fn kv_get(&self, k: &str) -> Option<String> {
        self.conn.lock()
            .query_row("SELECT v FROM kv WHERE k=?", params![k], |r| r.get::<_, String>(0))
            .ok()
    }
    pub fn kv_set(&self, k: &str, v: &str) -> Result<()> {
        self.conn.lock().execute(
            "INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v",
            params![k, v],
        )?;
        Ok(())
    }

    pub fn device_get(&self) -> Option<DeviceRow> {
        self.conn.lock()
            .query_row(
                "SELECT id, device_code, device_password_enc, token_enc, name, platform, os_version, app_version, allow_anonymous, require_device_code, accept_connections, last_ip, last_seen, created_at, online FROM device LIMIT 1",
                [],
                |r| {
                    let pw_enc: String = r.get(2)?;
                    let tok_enc: String = r.get(3)?;
                    let sec = self.sec.clone();
                    let pw = sec.decrypt(&pw_enc).unwrap_or_default();
                    let tok = sec.decrypt(&tok_enc).unwrap_or_default();
                    Ok(DeviceRow {
                        id: r.get(0)?,
                        device_code: r.get(1)?,
                        device_password: pw,
                        token: tok,
                        name: r.get(4)?,
                        platform: r.get(5)?,
                        os_version: r.get(6)?,
                        app_version: r.get(7)?,
                        allow_anonymous: r.get::<_, i64>(8)? != 0,
                        require_device_code: r.get::<_, i64>(9)? != 0,
                        accept_connections: r.get::<_, i64>(10)? != 0,
                        last_ip: r.get(11)?,
                        last_seen: r.get(12)?,
                        created_at: r.get(13)?,
                        online: r.get::<_, i64>(14)? != 0,
                    })
                },
            )
            .ok()
    }

    pub fn device_put(&self, d: &DeviceRow) -> Result<()> {
        let pw_enc = self.sec.encrypt(&d.device_password)?;
        let tok_enc = self.sec.encrypt(&d.token)?;
        let conn = self.conn.lock();
        conn.execute(
            "INSERT INTO device(id, device_code, device_password_enc, token_enc, name, platform, os_version, app_version, allow_anonymous, require_device_code, accept_connections, last_ip, last_seen, created_at, online)
             VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
             ON CONFLICT(id) DO UPDATE SET
                device_code=excluded.device_code,
                device_password_enc=excluded.device_password_enc,
                token_enc=excluded.token_enc,
                name=excluded.name,
                platform=excluded.platform,
                os_version=excluded.os_version,
                app_version=excluded.app_version,
                allow_anonymous=excluded.allow_anonymous,
                require_device_code=excluded.require_device_code,
                accept_connections=excluded.accept_connections,
                last_ip=excluded.last_ip,
                last_seen=excluded.last_seen,
                online=excluded.online
            ",
            params![
                d.id, d.device_code, pw_enc, tok_enc, d.name, d.platform, d.os_version, d.app_version,
                d.allow_anonymous as i64, d.require_device_code as i64, d.accept_connections as i64,
                d.last_ip, d.last_seen, d.created_at, d.online as i64,
            ],
        )?;
        Ok(())
    }
    pub fn device_clear(&self) -> Result<()> {
        self.conn.lock().execute("DELETE FROM device", [])?;
        Ok(())
    }
    pub fn device_update_flags(&self, allow: bool, req: bool, accept: bool) -> Result<()> {
        self.conn.lock().execute(
            "UPDATE device SET allow_anonymous=?, require_device_code=?, accept_connections=?",
            params![allow as i64, req as i64, accept as i64],
        )?;
        Ok(())
    }
    pub fn device_update_token(&self, token: &str) -> Result<()> {
        let tok_enc = self.sec.encrypt(token)?;
        self.conn.lock().execute("UPDATE device SET token_enc=?", params![tok_enc])?;
        Ok(())
    }
    pub fn device_set_password(&self, pw: &str) -> Result<()> {
        let pw_enc = self.sec.encrypt(pw)?;
        self.conn.lock().execute("UPDATE device SET device_password_enc=?", params![pw_enc])?;
        Ok(())
    }
    pub fn device_set_code(&self, code: &str) -> Result<()> {
        self.conn.lock().execute("UPDATE device SET device_code=?", params![code])?;
        Ok(())
    }
    // 文件传输状态
    pub fn file_upsert(&self, id: &str, direction: &str, transfer_id: &str, controller_id: &str,
                       controlled_code: &str, name: &str, size: i64, sha256: &str, chunk_size: i64,
                       file_path: &str) -> Result<()> {
        let now = chrono::Utc::now().timestamp();
        self.conn.lock().execute(
            "INSERT INTO file_transfers(id, direction, transfer_id, controller_id, controlled_code, name, size, sha256_expected, chunk_size, status, file_path, created_at, updated_at, received_offset)
             VALUES(?,?,?,?,?,?,?,?,?,'open',?,?,?,0)
             ON CONFLICT(id) DO UPDATE SET updated_at=excluded.updated_at",
            params![id, direction, transfer_id, controller_id, controlled_code, name, size, sha256, chunk_size, file_path, now, now],
        )?;
        Ok(())
    }
    pub fn file_progress(&self, id: &str, offset: i64, status: &str) -> Result<()> {
        let now = chrono::Utc::now().timestamp();
        self.conn.lock().execute(
            "UPDATE file_transfers SET received_offset=?, status=COALESCE(NULLIF(?, ''), status), updated_at=? WHERE id=?",
            params![offset, status, now, id],
        )?;
        Ok(())
    }
    /// 按 transfer_id 查最新 open 记录，用于断点续传
    pub fn file_find_open(&self, transfer_id: &str) -> Option<FileRow> {
        self.conn.lock()
            .query_row(
                "SELECT id, direction, transfer_id, controller_id, controlled_code, name, size, sha256_expected, chunk_size, received_offset, status, file_path FROM file_transfers WHERE transfer_id=? AND status='open' ORDER BY updated_at DESC LIMIT 1",
                params![transfer_id],
                |r| Ok(FileRow {
                    id: r.get(0)?,
                    direction: r.get(1)?,
                    transfer_id: r.get(2)?,
                    controller_id: r.get(3)?,
                    controlled_code: r.get(4)?,
                    name: r.get(5)?,
                    size: r.get(6)?,
                    sha256_expected: r.get(7)?,
                    chunk_size: r.get(8)?,
                    received_offset: r.get(9)?,
                    status: r.get(10)?,
                    file_path: r.get(11)?,
                }),
            )
            .ok()
    }
}

fn migrate_add_col(conn: &Connection, table: &str, col: &str, decl: &str) -> Result<()> {
    // pragma_table_info 用 ? 不能 bind，用 format 限定表名列名（table/col 来自内部常量，安全）
    let exists: i64 = conn.query_row(
        &format!("SELECT COUNT(*) FROM pragma_table_info('{}') WHERE name=?", table),
        params![col],
        |r| r.get(0),
    ).unwrap_or(0);
    if exists == 0 {
        conn.execute(&format!("ALTER TABLE {} ADD COLUMN {} {}", table, col, decl), [])?;
    }
    Ok(())
}

fn migrate_encrypt_legacy(conn: &Connection) -> Result<()> {
    // 检查是否还有 legacy 平文本列
    let has_legacy_pw: i64 = conn.query_row(
        "SELECT COUNT(*) FROM pragma_table_info('device') WHERE name='device_password'",
        [],
        |r| r.get(0),
    ).unwrap_or(0);
    let has_legacy_tok: i64 = conn.query_row(
        "SELECT COUNT(*) FROM pragma_table_info('device') WHERE name='token'",
        [],
        |r| r.get(0),
    ).unwrap_or(0);
    if has_legacy_pw == 0 && has_legacy_tok == 0 {
        return Ok(());
    }
    // 实例化一个临时 SecureStore 加密后回写
    let sec = SecureStore::new()?;
    if has_legacy_pw == 1 {
        let mut stmt = conn.prepare("SELECT id, device_password FROM device")?;
        let rows: Vec<(i64, String)> = stmt.query_map([], |r| Ok((r.get::<_, i64>(0)?, r.get::<_, String>(1)?)))?
            .filter_map(|x| x.ok()).collect();
        for (id, pw) in rows {
            if pw.is_empty() { continue; }
            let enc = sec.encrypt(&pw).unwrap_or_default();
            conn.execute("UPDATE device SET device_password_enc=? WHERE id=?", params![enc, id])?;
        }
    }
    if has_legacy_tok == 1 {
        let mut stmt = conn.prepare("SELECT id, token FROM device")?;
        let rows: Vec<(i64, String)> = stmt.query_map([], |r| Ok((r.get::<_, i64>(0)?, r.get::<_, String>(1)?)))?
            .filter_map(|x| x.ok()).collect();
        for (id, tok) in rows {
            if tok.is_empty() { continue; }
            let enc = sec.encrypt(&tok).unwrap_or_default();
            conn.execute("UPDATE device SET token_enc=? WHERE id=?", params![enc, id])?;
        }
    }
    Ok(())
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct FileRow {
    pub id: String,
    pub direction: String,
    pub transfer_id: String,
    pub controller_id: Option<String>,
    pub controlled_code: Option<String>,
    pub name: String,
    pub size: i64,
    pub sha256_expected: Option<String>,
    pub chunk_size: i64,
    pub received_offset: i64,
    pub status: String,
    pub file_path: Option<String>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct DeviceRow {
    pub id: i64,
    pub device_code: String,
    pub device_password: String,
    pub token: String,
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
}

pub fn data_dir() -> PathBuf {
    if let Some(p) = dirs::data_local_dir() {
        return p.join("LinkALL Hosted");
    }
    PathBuf::from(".")
}

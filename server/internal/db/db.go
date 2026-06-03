package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Open(path string) error {
	var err error
	DB, err = sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	DB.SetMaxOpenConns(1) // SQLite 写串行化最稳
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Printf("[db] ready at %s", path)
	return nil
}

func migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			is_super_admin INTEGER NOT NULL DEFAULT 0,
			banned INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_login_ip TEXT,
			last_login_at INTEGER,
			locale TEXT DEFAULT 'zh-CN',
			avatar TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS invites(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT UNIQUE NOT NULL,
			created_by INTEGER NOT NULL,
			used_by INTEGER,
			max_uses INTEGER NOT NULL DEFAULT 1,
			used_count INTEGER NOT NULL DEFAULT 0,
			ttl_hours INTEGER NOT NULL DEFAULT 72,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			note TEXT,
			FOREIGN KEY(created_by) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS devices(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id INTEGER,
			device_code TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT,
			platform TEXT,
			os_version TEXT,
			app_version TEXT,
			allow_anonymous INTEGER NOT NULL DEFAULT 1,
			require_device_code INTEGER NOT NULL DEFAULT 1,
			accept_connections INTEGER NOT NULL DEFAULT 1,
			last_ip TEXT,
			last_seen INTEGER,
			created_at INTEGER NOT NULL,
			online INTEGER NOT NULL DEFAULT 0,
			tag TEXT,
			notes TEXT,
			FOREIGN KEY(owner_id) REFERENCES users(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_owner ON devices(owner_id)`,
		`CREATE TABLE IF NOT EXISTS device_sessions(
			id TEXT PRIMARY KEY,
			controller_id TEXT,
			controlled_id INTEGER NOT NULL,
			started_at INTEGER NOT NULL,
			last_active INTEGER NOT NULL,
			closed INTEGER NOT NULL DEFAULT 0,
			bytes_tx INTEGER NOT NULL DEFAULT 0,
			bytes_rx INTEGER NOT NULL DEFAULT 0,
			relay_used INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(controlled_id) REFERENCES devices(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_device ON device_sessions(controlled_id)`,
		`CREATE TABLE IF NOT EXISTS announcements(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			author_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content_md TEXT NOT NULL,
			platform TEXT,
			min_version TEXT,
			pinned INTEGER NOT NULL DEFAULT 0,
			force_read INTEGER NOT NULL DEFAULT 0,
			signature TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(author_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS announcement_reads(
			announcement_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			read_at INTEGER NOT NULL,
			PRIMARY KEY(announcement_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS ota_packages(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			version TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT 'stable',
			file_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			signature TEXT,
			release_notes TEXT,
			force_update INTEGER NOT NULL DEFAULT 0,
			min_supported_version TEXT,
			downloads INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			UNIQUE(platform, version, channel)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_id INTEGER,
			action TEXT NOT NULL,
			target TEXT,
			ip TEXT,
			detail TEXT,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action_time ON audit_logs(action, created_at)`,
		`CREATE TABLE IF NOT EXISTS settings(
			k TEXT PRIMARY KEY,
			v TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		// ===== 安全相关新增 =====
		// 登录失败 / 账号锁定
		`CREATE TABLE IF NOT EXISTS login_attempts(
			username TEXT PRIMARY KEY,
			failed_attempts INTEGER NOT NULL DEFAULT 0,
			last_fail_at INTEGER NOT NULL DEFAULT 0,
			locked_at INTEGER NOT NULL DEFAULT 0,
			last_ip TEXT
		)`,
		// JWT 多密钥（kid → secret）
		`CREATE TABLE IF NOT EXISTS jwt_keys(
			kid TEXT PRIMARY KEY,
			secret TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			active INTEGER NOT NULL DEFAULT 0
		)`,
		// 文件传输状态（断点续传）
		`CREATE TABLE IF NOT EXISTS file_transfers(
			id TEXT PRIMARY KEY,
			direction TEXT NOT NULL,            -- 'c2h' 控制器→被控 / 'h2c' 被控→控制器
			transfer_id TEXT NOT NULL,          -- 应用层 transfer_id
			controller_id TEXT,
			controlled_code TEXT,
			name TEXT NOT NULL,
			size INTEGER NOT NULL,
			sha256_expected TEXT,
			chunk_size INTEGER NOT NULL DEFAULT 262144,
			received_offset INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'open', -- open / completed / aborted
			file_path TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ft_controller ON file_transfers(controller_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ft_controlled ON file_transfers(controlled_code)`,
		// WebSocket 反重放 nonce 记录
		`CREATE TABLE IF NOT EXISTS ws_nonces(
			nonce TEXT PRIMARY KEY,
			expire_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_nonces_expire ON ws_nonces(expire_at)`,
		// 客户端崩溃 / 日志上报（用于远程诊断）
		`CREATE TABLE IF NOT EXISTS crash_logs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_id INTEGER,
			device_code TEXT,
			platform TEXT NOT NULL,
			app_version TEXT,
			os_version TEXT,
			level TEXT NOT NULL,
			source TEXT,
			message TEXT NOT NULL,
			stack TEXT,
			extra TEXT,
			client_ip TEXT,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_crash_actor ON crash_logs(actor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_crash_device ON crash_logs(device_code)`,
		`CREATE INDEX IF NOT EXISTS idx_crash_created ON crash_logs(created_at)`,
		// FCM device tokens（Android controlled 用于唤醒）
		`CREATE TABLE IF NOT EXISTS fcm_tokens(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_code TEXT NOT NULL,
			token TEXT NOT NULL,
			platform TEXT NOT NULL,
			app_version TEXT,
			created_at INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fcm_token ON fcm_tokens(token)`,
		`CREATE INDEX IF NOT EXISTS idx_fcm_device ON fcm_tokens(device_code)`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("stmt: %s -> %w", s[:40], err)
		}
	}
	// 限流策略预置（中等）
	upsertSetting("rate_limit_strictness", "medium")
	upsertSetting("rate_limit_login", "5|900")
	upsertSetting("rate_limit_register", "3|3600")
	upsertSetting("rate_limit_device_register", "5|3600")
	upsertSetting("rate_limit_ws_connect", "30|60")
	upsertSetting("rate_limit_ws_cmd", "30|1")
	upsertSetting("rate_limit_ws_file", "60|1")
	upsertSetting("ws_replay_window_sec", "30")
	upsertSetting("ws_max_message_kb", "1024")
	upsertSetting("ota_pubkey_b64", "") // 由 Signer 在首次启动生成并写入
	upsertSetting("ota_privkey_b64", "")
	upsertSetting("ota_keyid", "")
	upsertSetting("allow_origins_csv", "")
	return nil
}

func upsertSetting(k, v string) {
	DB.Exec(
		`INSERT INTO settings(k, v, updated_at) VALUES(?,?,strftime('%s','now'))
		 ON CONFLICT(k) DO UPDATE SET v=excluded.v, updated_at=excluded.updated_at`,
		k, v,
	)
}

func GetSetting(k string) string {
	var v string
	_ = DB.QueryRow(`SELECT v FROM settings WHERE k=?`, k).Scan(&v)
	return v
}

func GetSettingInt(k string, def int) int {
	v := GetSetting(k)
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return def
		}
	}
	return n
}

func SetSetting(k, v string) error {
	_, err := DB.Exec(
		`INSERT INTO settings(k, v, updated_at) VALUES(?,?,strftime('%s','now'))
		 ON CONFLICT(k) DO UPDATE SET v=excluded.v, updated_at=excluded.updated_at`,
		k, v,
	)
	return err
}

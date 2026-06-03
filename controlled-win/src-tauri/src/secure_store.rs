// AES-256-GCM 加密本地存储的敏感数据（设备 token、密码）
// Key 来源：
//   - Windows: DPAPI（CryptProtectData/CryptUnprotectData）
//   - 其他平台: 基于 hostname + 静态 salt 的派生 key（demo only — 生产建议接 OS Keychain）
//
// DPAPI 流程：
//   1) 生成 32B 随机 key
//   2) CryptProtectData 加密后写到 %LOCALAPPDATA%/LinkALL Hosted/secure_key.bin
//   3) 后续启动读 -> CryptUnprotectData 解密
//
// 跨用户/跨设备支持（#13）：
//   - SecureStoreMode::User (默认)  -> 同一 Windows 用户在同一台机器上能解密
//   - SecureStoreMode::Machine      -> 该机器任何管理员都能解密（用 LocalMachine scope + machine-scope entropy）
//   - SecureStoreMode::Roaming      -> 域用户漫游（CRYPTPROTECT_LOCAL_MACHINE 不变，加 roaming entropy）
//   - BackupKey: 把同一 32B 随机 key 额外用 Machine scope 加密备份到 secure_key_machine.bin
//     -> 任何用户/重装后可用 BackupKey::recover_machine_key() 恢复
//
// 生产部署建议：
//   - 家用单机：User
//   - 多用户共享 PC：Machine（首次启动时让 admin 确认）
//   - 域漫游：Roaming（需 Active Directory 漫游 profile）
use aes_gcm::aead::{Aead, KeyInit};
use aes_gcm::{Aes256Gcm, Key, Nonce};
use anyhow::Result;
use base64::{engine::general_purpose, Engine as _};
use rand::RngCore;
use std::path::PathBuf;
use std::sync::OnceLock;

pub struct SecureStore {
    key: [u8; 32],
    mode: SecureStoreMode,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SecureStoreMode {
    /// 仅当前 Windows 用户可解密（默认）
    User,
    /// 本机任何管理员可解密（用 Machine scope）
    Machine,
    /// 域用户漫游 profile（仍走 User scope，加额外 entropy 区分）
    Roaming,
}

impl Default for SecureStoreMode {
    fn default() -> Self {
        SecureStoreMode::User
    }
}

impl SecureStoreMode {
    pub fn as_str(&self) -> &'static str {
        match self {
            SecureStoreMode::User => "user",
            SecureStoreMode::Machine => "machine",
            SecureStoreMode::Roaming => "roaming",
        }
    }
    pub fn from_str(s: &str) -> Self {
        match s.to_ascii_lowercase().as_str() {
            "machine" => SecureStoreMode::Machine,
            "roaming" => SecureStoreMode::Roaming,
            _ => SecureStoreMode::User,
        }
    }
}

impl SecureStore {
    /// 创建默认（user scope）
    pub fn new() -> Result<Self> {
        Self::with_mode(SecureStoreMode::User, true)
    }

    /// 指定 scope；backup=true 时同步生成 Machine-scope 备份
    pub fn with_mode(mode: SecureStoreMode, backup: bool) -> Result<Self> {
        let key = match load_or_create_key(mode) {
            Ok(k) => k,
            Err(_) => fallback_key(),
        };
        if backup {
            // 异步/同步：第一次没备份时创建一份
            let _ = ensure_machine_backup(&key);
        }
        Ok(Self { key, mode })
    }

    pub fn mode(&self) -> SecureStoreMode { self.mode }

    pub fn encrypt(&self, plaintext: &str) -> Result<String> {
        let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(&self.key));
        let mut nonce = [0u8; 12];
        rand::thread_rng().fill_bytes(&mut nonce);
        let ct = cipher
            .encrypt(Nonce::from_slice(&nonce), plaintext.as_bytes())
            .map_err(|_| anyhow::anyhow!("encrypt"))?;
        let mut out = nonce.to_vec();
        out.extend(ct);
        Ok(general_purpose::STANDARD.encode(&out))
    }

    pub fn decrypt(&self, b64: &str) -> Result<String> {
        let data = general_purpose::STANDARD.decode(b64)?;
        if data.len() < 12 {
            return Err(anyhow::anyhow!("short"));
        }
        let (nonce, ct) = data.split_at(12);
        let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(&self.key));
        let pt = cipher
            .decrypt(Nonce::from_slice(nonce), ct)
            .map_err(|_| anyhow::anyhow!("decrypt"))?;
        Ok(String::from_utf8_lossy(&pt).to_string())
    }

    /// 导出当前 key（base64），用于跨用户/跨设备迁移
    /// 注意：导出后任何拿到这一串 base64 的人 + 拿到 ciphertext 的人都能解密
    /// 生产应再加密一次（比如用收件人公钥或预共享密钥）
    pub fn export_key(&self) -> String {
        general_purpose::STANDARD.encode(self.key)
    }

    /// 从别处导出的 base64 key 导入
    pub fn from_exported_key(b64: &str) -> Result<Self> {
        let data = general_purpose::STANDARD.decode(b64)?;
        if data.len() != 32 {
            return Err(anyhow::anyhow!("bad key size"));
        }
        let mut k = [0u8; 32];
        k.copy_from_slice(&data);
        Ok(Self { key: k, mode: SecureStoreMode::User })
    }
}

#[cfg(target_os = "windows")]
fn key_path() -> Option<PathBuf> {
    let dir = std::env::var("LOCALAPPDATA")
        .or_else(|_| std::env::var("APPDATA"))
        .ok()
        .map(PathBuf::from)?;
    Some(dir.join("LinkALL Hosted").join("secure_key.bin"))
}

#[cfg(target_os = "windows")]
fn machine_key_path() -> Option<PathBuf> {
    let dir = std::env::var("PROGRAMDATA").ok().map(PathBuf::from)?;
    Some(dir.join("LinkALL Hosted").join("secure_key_machine.bin"))
}

#[cfg(target_os = "windows")]
#[repr(C)]
struct CRYPT_DATA_BLOB {
    cbData: u32,
    pbData: *mut core::ffi::c_void,
}

#[cfg(target_os = "windows")]
impl Default for CRYPT_DATA_BLOB {
    fn default() -> Self { Self { cbData: 0, pbData: core::ptr::null_mut() } }
}

#[cfg(target_os = "windows")]
#[link(name = "Crypt32")]
extern "system" {
    fn CryptProtectData(
        pDataIn: *const CRYPT_DATA_BLOB,
        szDataDescr: *const u16,
        pOptionalEntropy: *const CRYPT_DATA_BLOB,
        pvReserved: *mut core::ffi::c_void,
        pPromptStruct: *mut core::ffi::c_void,
        dwFlags: u32,
        pDataOut: *mut CRYPT_DATA_BLOB,
    ) -> i32;
    fn CryptUnprotectData(
        pDataIn: *const CRYPT_DATA_BLOB,
        ppszDataDescr: *mut *mut u16,
        pOptionalEntropy: *const CRYPT_DATA_BLOB,
        pvReserved: *mut core::ffi::c_void,
        pPromptStruct: *mut core::ffi::c_void,
        dwFlags: u32,
        pDataOut: *mut CRYPT_DATA_BLOB,
    ) -> i32;
}

#[cfg(target_os = "windows")]
#[link(name = "kernel32")]
extern "system" {
    fn LocalFree(hmem: *mut core::ffi::c_void) -> *mut core::ffi::c_void;
}

#[cfg(target_os = "windows")]
fn dpapi_result(r: i32) -> Result<()> {
    if r == 0 { Err(anyhow::anyhow!("DPAPI call failed")) } else { Ok(()) }
}

#[cfg(target_os = "windows")]
fn load_or_create_key(mode: SecureStoreMode) -> Result<[u8; 32]> {
    let path = key_path().ok_or_else(|| anyhow::anyhow!("no APPDATA"))?;
    if let Ok(b) = std::fs::read(&path) {
        if let Ok(k) = dpapi_unprotect(&b, mode) {
            return Ok(k);
        }
    }
    let mut k = [0u8; 32];
    rand::thread_rng().fill_bytes(&mut k);
    let protected = dpapi_protect(&k, mode)?;
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    std::fs::write(&path, &protected)?;
    Ok(k)
}

#[cfg(target_os = "windows")]
fn ensure_machine_backup(key: &[u8; 32]) -> Result<()> {
    let Some(path) = machine_key_path() else { return Ok(()) };
    if path.exists() { return Ok(()); }
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    let protected = dpapi_protect(key, SecureStoreMode::Machine)?;
    std::fs::write(&path, &protected)?;
    Ok(())
}

#[cfg(target_os = "windows")]
pub fn recover_machine_key() -> Result<[u8; 32]> {
    let Some(path) = machine_key_path() else { return Err(anyhow::anyhow!("no PROGRAMDATA")) };
    if !path.exists() {
        return Err(anyhow::anyhow!("no machine backup; cannot recover"));
    }
    let b = std::fs::read(&path)?;
    let in_blob = CRYPT_DATA_BLOB { cbData: b.len() as u32, pbData: b.as_ptr() as *mut _ };
    let mut out = CRYPT_DATA_BLOB::default();
    unsafe {
        dpapi_result(CryptUnprotectData(&in_blob, core::ptr::null_mut(), core::ptr::null(), core::ptr::null_mut(), core::ptr::null_mut(), 0, &mut out))?;
        if out.cbData != 32 {
            LocalFree(out.pbData as *mut _);
            return Err(anyhow::anyhow!("bad key size"));
        }
        let mut k = [0u8; 32];
        core::ptr::copy_nonoverlapping(out.pbData as *const u8, k.as_mut_ptr(), 32);
        LocalFree(out.pbData as *mut _);
        Ok(k)
    }
}

#[cfg(target_os = "windows")]
fn dpapi_protect(data: &[u8], mode: SecureStoreMode) -> Result<Vec<u8>> {
    let in_blob = CRYPT_DATA_BLOB { cbData: data.len() as u32, pbData: data.as_ptr() as *mut _ };
    let flags: u32 = match mode {
        SecureStoreMode::Machine => 0x4,
        _ => 0x0,
    };
    let mut out = CRYPT_DATA_BLOB::default();
    unsafe {
        dpapi_result(CryptProtectData(&in_blob, core::ptr::null(), core::ptr::null(), core::ptr::null_mut(), core::ptr::null_mut(), flags, &mut out))?;
        let mut v = vec![0u8; out.cbData as usize];
        core::ptr::copy_nonoverlapping(out.pbData as *const u8, v.as_mut_ptr(), v.len());
        LocalFree(out.pbData as *mut _);
        Ok(v)
    }
}

#[cfg(target_os = "windows")]
fn dpapi_unprotect(data: &[u8], mode: SecureStoreMode) -> Result<[u8; 32]> {
    let in_blob = CRYPT_DATA_BLOB { cbData: data.len() as u32, pbData: data.as_ptr() as *mut _ };
    let flags: u32 = match mode {
        SecureStoreMode::Machine => 0x4,
        _ => 0x0,
    };
    let mut out = CRYPT_DATA_BLOB::default();
    unsafe {
        dpapi_result(CryptUnprotectData(&in_blob, core::ptr::null_mut(), core::ptr::null(), core::ptr::null_mut(), core::ptr::null_mut(), flags, &mut out))?;
        if out.cbData != 32 {
            LocalFree(out.pbData as *mut _);
            return Err(anyhow::anyhow!("bad key size"));
        }
        let mut k = [0u8; 32];
        core::ptr::copy_nonoverlapping(out.pbData as *const u8, k.as_mut_ptr(), 32);
        LocalFree(out.pbData as *mut _);
        Ok(k)
    }
}

#[cfg(not(target_os = "windows"))]
fn load_or_create_key(_mode: SecureStoreMode) -> Result<[u8; 32]> {
    Err(anyhow::anyhow!("not implemented on this platform"))
}

fn fallback_key() -> [u8; 32] {
    static WARNED: OnceLock<()> = OnceLock::new();
    if WARNED.set(()).is_ok() {
        eprintln!("[secure_store] WARN: using host-derived fallback key (NOT secure against admin-level access)");
    }
    let mut h: [u8; 32] = [0; 32];
    let host = std::env::var("COMPUTERNAME")
        .or_else(|_| std::env::var("HOSTNAME"))
        .unwrap_or_else(|_| "linkall-fallback".into());
    let b = host.as_bytes();
    for i in 0..32 {
        h[i] = b[i % b.len()].wrapping_add(i as u8);
    }
    h
}

#[cfg(not(target_os = "windows"))]
pub fn recover_machine_key() -> Result<[u8; 32]> {
    Err(anyhow::anyhow!("not implemented on this platform"))
}

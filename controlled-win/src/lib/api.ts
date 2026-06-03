// Tauri 2 invoke 包装（被控端只调用后端命令）
import { invoke } from '@tauri-apps/api/core';

export interface DeviceInfo {
  id: number;
  device_code: string;
  name: string;
  platform: string;
  os_version: string;
  app_version: string;
  allow_anonymous: boolean;
  require_device_code: boolean;
  accept_connections: boolean;
  last_ip: string;
  last_seen: number;
  created_at: number;
  online: boolean;
  token: string;
}

export interface ServerConfig {
  public_url: string;
  official_server: string;
  ice_servers: Array<{ urls: string; username?: string; credential?: string }>;
}

export interface ConnectionInfo {
  id: string;
  from: string;
  device_code: string;
  ts: number;
  mode: string;
}

export const api = {
  // 配置
  getServer: () => invoke<ServerConfig>('get_server'),
  setServer: (url: string) => invoke<void>('set_server', { url }),
  getLocale: () => invoke<string>('get_locale'),
  setLocale: (l: string) => invoke<void>('set_locale', { locale: l }),

  // 设备
  registerDevice: (req: any) => invoke<DeviceInfo>('register_device', { req }),
  loginDevice: (req: any) => invoke<DeviceInfo>('login_device', { req }),
  getDevice: () => invoke<DeviceInfo | null>('get_device'),
  updateFlags: (allowAn: boolean, reqCode: boolean, accept: boolean) =>
    invoke<DeviceInfo>('update_flags', { allowAn, reqCode, accept }),
  resetCode: (newCode: string, newPw: string) => invoke<DeviceInfo>('reset_code', { newCode, newPw }),
  logoutDevice: () => invoke<void>('logout_device'),

  // 状态
  getStatus: () => invoke<{running: boolean; signaling: string; last_error: string; screen_w: number; screen_h: number}>('get_status'),
  startService: () => invoke<void>('start_service'),
  stopService: () => invoke<void>('stop_service'),

  // 自启/系统
  setAutostart: (enable: boolean) => invoke<void>('set_autostart', { enable }),
  getAutostart: () => invoke<boolean>('get_autostart'),
  quit: () => invoke<void>('quit_app'),
  showMain: () => invoke<void>('show_main'),

  // 多显示器
  listDisplays: () => invoke<Array<{index: number; width: number; height: number; is_primary: boolean; name: string}>>('list_displays'),
  selectDisplay: (idx: number) => invoke<void>('select_display', { idx }),
  getSelectedDisplay: () => invoke<number>('get_selected_display'),

  // ICE servers
  getIceServers: () => invoke<Array<{urls: string; username?: string; credential?: string}>>('get_ice_servers'),

  // 日志 / 崩溃
  logWrite: (level: string, message: string, extra?: string) =>
    invoke<void>('log_write', { level, message, extra }),
  reportCrash: (level: string, message: string, stack?: string, source?: string) =>
    invoke<void>('report_crash', { level, message, stack, source }),
  uploadLogs: (limit?: number) => invoke<number>('upload_logs', { limit }),

  // 硬件 H.264 能力
  getHwCapability: () => invoke<{ backend: string; max_width: number; max_height: number; max_fps: number; note: string; driver_version: string }>('get_hw_capability'),
  reProbeHw: () => invoke<{ backend: string; note: string }>('re_probe_hw'),

  // SecureStore 跨用户
  secureStoreMode: () => invoke<string>('secure_store_mode'),
  exportSecureKey: () => invoke<string>('export_secure_key'),
  importSecureKey: (b64: string) => invoke<string>('import_secure_key', { b64 }),
  recoverFromMachineBackup: () => invoke<string>('recover_from_machine_backup'),
};

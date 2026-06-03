// 极简 fetch 封装；带 JWT；统一错误处理
const BASE = '';  // 走 Vite proxy → :8080

let token = $state<string | null>(localStorage.getItem('linkall.admin.jwt'));

export function getToken() { return token; }
export function setToken(t: string | null) {
  token = t;
  if (t) localStorage.setItem('linkall.admin.jwt', t);
  else localStorage.removeItem('linkall.admin.jwt');
}

export class ApiError extends Error {
  constructor(public status: number, public body: any, msg: string) { super(msg); }
}

async function request<T>(method: string, path: string, body?: any, qs?: Record<string, any>): Promise<T> {
  let url = BASE + path;
  if (qs) {
    const u = new URLSearchParams();
    for (const [k, v] of Object.entries(qs)) {
      if (v !== undefined && v !== null && v !== '') u.set(k, String(v));
    }
    const s = u.toString();
    if (s) url += '?' + s;
  }
  const init: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  };
  const r = await fetch(url, init);
  const text = await r.text();
  let data: any;
  try { data = text ? JSON.parse(text) : {}; } catch { data = { raw: text }; }
  if (!r.ok) throw new ApiError(r.status, data, data?.error || r.statusText);
  return data as T;
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ token: string; user: any; exp: number }>('POST', '/api/auth/login', { username, password }),
  me: () => request<{ user: any }>('GET', '/api/auth/me'),

  // Stats
  stats: () => request<any>('GET', '/api/admin/stats'),

  // Crash logs
  crashLogs: (q: { level?: string; device_code?: string; from?: number; to?: number; limit?: number } = {}) =>
    request<{ logs: CrashLog[] }>('GET', '/api/admin/crash-logs', undefined, q),
  crashStats: () => request<{ total: number; by_level: Record<string, number> }>('GET', '/api/admin/crash-logs/stats'),

  // Devices
  devices: () => request<{ devices: any[] }>('GET', '/api/devices'),
  wake: (device_id: string, method: string = 'both') =>
    request<any>('POST', '/api/admin/wake', { device_id, method }),
  revokeDevice: (id: string) => request<{ ok: boolean }>('DELETE', '/api/devices/' + id),

  // FCM tokens
  fcmTokens: (limit = 100) => request<{ tokens: FcmToken[] }>('GET', '/api/admin/fcm-tokens', undefined, { limit }),

  // Rate limit
  getRateLimit: () => request<{ strictness: string; overrides: Record<string, any> }>('GET', '/api/admin/rate-limit'),
  setRateLimit: (strictness: string) => request<{ ok: boolean }>('PUT', '/api/admin/rate-limit', { strictness }),

  // Audit
  auditLogs: (limit = 200) => request<{ logs: any[] }>('GET', '/api/admin/audit-logs', undefined, { limit }),
};

export interface CrashLog {
  id: string;
  device_code: string;
  platform: string;
  app_version: string;
  os_version: string;
  level: string;
  source: string;
  message: string;
  stack: string;
  extra: string;
  client_ip: string;
  created_at: number;
}

export interface FcmToken {
  id: string;
  device_code: string;
  platform: string;
  app_version: string;
  last_seen: number;
  revoked: boolean;
}

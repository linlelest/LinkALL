import { get } from 'svelte/store';
import { token, user, logout } from './auth';

const SERVER_KEY = 'linkall.server';

function getServer(): string {
  try {
    const s = localStorage.getItem(SERVER_KEY);
    if (s) return s.replace(/\/+$/, '');
  } catch {}
  return '';
}

export function setServer(url: string) {
  try { localStorage.setItem(SERVER_KEY, url.replace(/\/+$/, '')); } catch {}
}

export function apiBase(): string {
  const s = getServer();
  if (s) return s;
  // 默认同源
  if (typeof window !== 'undefined') return window.location.origin;
  return '';
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T = any>(path: string, init: RequestInit = {}): Promise<T> {
  const base = apiBase();
  const tk = get(token);
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init.headers as any || {})
  };
  if (tk) headers['Authorization'] = `Bearer ${tk}`;
  let res: Response;
  try {
    res = await fetch(base + path, { ...init, headers });
  } catch (e: any) {
    throw new ApiError(0, '网络错误: ' + (e?.message || e));
  }
  if (res.status === 401) {
    logout();
    throw new ApiError(401, '未登录');
  }
  const text = await res.text();
  let data: any;
  try { data = text ? JSON.parse(text) : {}; } catch { data = { raw: text }; }
  if (!res.ok) {
    throw new ApiError(res.status, data?.error || `HTTP ${res.status}`);
  }
  return data as T;
}

export const api = {
  get:  (p: string) => request(p),
  post: (p: string, body: any) => request(p, { method: 'POST', body: JSON.stringify(body) }),
  patch:(p: string, body: any) => request(p, { method: 'PATCH', body: JSON.stringify(body) }),
  del:  (p: string) => request(p, { method: 'DELETE' }),
  upload: async (p: string, form: FormData) => {
    const tk = get(token);
    const headers: Record<string, string> = {};
    if (tk) headers['Authorization'] = `Bearer ${tk}`;
    const res = await fetch(apiBase() + p, { method: 'POST', body: form, headers });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new ApiError(res.status, data?.error || `HTTP ${res.status}`);
    return data;
  }
};

export { user, token };

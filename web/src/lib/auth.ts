import { writable, type Writable } from 'svelte/store';

export interface AuthUser {
  id: number;
  username: string;
  is_admin: boolean;
  is_super_admin: boolean;
  locale?: string;
  avatar?: string;
}

const TOKEN_KEY = 'linkall.token';
const USER_KEY = 'linkall.user';

export const token: Writable<string> = writable(load<string>(TOKEN_KEY, ''));
export const user: Writable<AuthUser | null> = writable(load<AuthUser | null>(USER_KEY, null));

token.subscribe((v) => save(TOKEN_KEY, v));
user.subscribe((v) => save(USER_KEY, v));

export function logout() {
  token.set('');
  user.set(null);
  try { localStorage.removeItem(TOKEN_KEY); localStorage.removeItem(USER_KEY); } catch {}
}

function load<T>(k: string, def: T): T {
  try {
    const s = localStorage.getItem(k);
    if (s == null) return def;
    return JSON.parse(s) as T;
  } catch { return def; }
}

function save(k: string, v: any) {
  try { localStorage.setItem(k, JSON.stringify(v)); } catch {}
}

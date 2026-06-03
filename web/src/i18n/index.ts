import { writable, derived, get, type Writable } from 'svelte/store';
import zhCN from './zh-CN.json';
import enUS from './en-US.json';

export type Locale = 'zh-CN' | 'en-US';

const dicts: Record<Locale, Record<string, string>> = {
  'zh-CN': zhCN,
  'en-US': enUS
};

function detectLocale(): Locale {
  if (typeof localStorage === 'undefined') return 'zh-CN';
  const s = localStorage.getItem('linkall.locale');
  if (s === 'zh-CN' || s === 'en-US') return s;
  const nav = (navigator.language || '').toLowerCase();
  if (nav.startsWith('en')) return 'en-US';
  return 'zh-CN';
}

export const locale: Writable<Locale> = writable(detectLocale());

export const t = derived(locale, ($l) => {
  return (key: string, fallback?: string): string => {
    const d = dicts[$l] || dicts['zh-CN'];
    return d[key] ?? fallback ?? key;
  };
});

export function setLocale(l: Locale) {
  locale.set(l);
  try { localStorage.setItem('linkall.locale', l); } catch {}
}

export function getLocale(): Locale {
  return get(locale);
}

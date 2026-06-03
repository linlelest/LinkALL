import { writable, type Writable } from 'svelte/store';

export type Route =
  | { name: 'login' }
  | { name: 'dashboard' }
  | { name: 'devices' }
  | { name: 'connect' }
  | { name: 'control'; deviceCode: string }
  | { name: 'announcements' }
  | { name: 'profile' }
  | { name: 'admin'; section?: 'users' | 'invites' | 'announcements' | 'ota' | 'server' };

function parseHash(): Route {
  const h = (typeof location !== 'undefined' ? location.hash : '').replace(/^#\/?/, '');
  if (!h) return { name: 'dashboard' };
  const [head, ...rest] = h.split('/');
  switch (head) {
    case 'login': return { name: 'login' };
    case 'dashboard': return { name: 'dashboard' };
    case 'devices': return { name: 'devices' };
    case 'connect': return { name: 'connect' };
    case 'control': return { name: 'control', deviceCode: rest[0] || '' };
    case 'announcements': return { name: 'announcements' };
    case 'profile': return { name: 'profile' };
    case 'admin': return { name: 'admin', section: (rest[0] as any) || 'users' };
    default: return { name: 'dashboard' };
  }
}

export const route: Writable<Route> = writable(parseHash());

if (typeof window !== 'undefined') {
  window.addEventListener('hashchange', () => route.set(parseHash()));
}

export function go(r: Route) {
  let h = '#/' + r.name;
  if (r.name === 'control') h += '/' + (r as any).deviceCode;
  if (r.name === 'admin' && (r as any).section) h += '/' + (r as any).section;
  if (location.hash !== h) location.hash = h;
  else route.set(r);
}

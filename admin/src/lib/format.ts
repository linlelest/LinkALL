// 通用格式化 helpers
export function formatBytes(b: any): string {
  if (b === null || b === undefined) return '-';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let n = Number(b);
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return n.toFixed(1) + ' ' + u[i];
}

export function fmtTime(ts: number | undefined | null): string {
  if (!ts) return '-';
  const d = new Date(Number(ts) * 1000);
  return d.toLocaleString();
}

export function badgeClass(l: string): string {
  return ({
    fatal: 'badge-fatal', error: 'badge-error', warn: 'badge-warn',
    info: 'badge-info', debug: 'badge-debug', trace: 'badge-trace',
  } as Record<string, string>)[l] || 'badge-debug';
}

export function fmtLevel(l: string): string {
  return ({ fatal: '🛑 fatal', error: '❌ error', warn: '⚠ warn', info: 'ℹ info', debug: '🔧 debug', trace: '· trace' } as Record<string,string>)[l] || l;
}

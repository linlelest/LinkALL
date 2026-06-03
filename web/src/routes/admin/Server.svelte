<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../../i18n';
  import { api } from '../../lib/api';

  let stats = $state<any>(null);
  let cfg = $state<any>(null);
  let timer: any;

  onMount(async () => {
    await refresh();
    timer = setInterval(refresh, 5000);
  });

  async function refresh() {
    try {
      stats = await api.get('/api/admin/stats');
      cfg = await api.get('/api/config');
    } catch {}
  }

  function fmtBytes(n: number) {
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
    if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
    return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB';
  }
  function fmtMem(n: number) { return fmtBytes(n); }
  function uptime() {
    if (!stats?.server_time) return '-';
    return new Date(stats.server_time * 1000).toLocaleString();
  }
</script>

<h1 class="text-xl font-semibold mb-4">{$t('admin.server.title')}</h1>

<div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.users')}</div>
    <div class="text-2xl font-bold mt-1">{stats?.users ?? '-'}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.devices')}</div>
    <div class="text-2xl font-bold mt-1">{stats?.devices ?? '-'}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.online')}</div>
    <div class="text-2xl font-bold mt-1 text-emerald-400">{stats?.online ?? '-'}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.sessions')}</div>
    <div class="text-2xl font-bold mt-1">{stats?.sessions ?? '-'}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.traffic.tx')}</div>
    <div class="text-xl font-bold mt-1">{fmtBytes(stats?.bytes_tx || 0)}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.traffic.rx')}</div>
    <div class="text-xl font-bold mt-1">{fmtBytes(stats?.bytes_rx || 0)}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.routines')}</div>
    <div class="text-xl font-bold mt-1">{stats?.go_routines ?? '-'}</div>
  </div>
  <div class="card p-3">
    <div class="text-xs text-dark-muted">{$t('admin.server.mem')}</div>
    <div class="text-xl font-bold mt-1">{fmtMem(stats?.go_mem_alloc || 0)}</div>
  </div>
</div>

<div class="card p-4 mb-3 text-sm space-y-1">
  <div><span class="text-dark-muted">{$t('admin.server.runtime')}:</span> <code class="text-primary-300">{stats?.go_version}</code></div>
  <div><span class="text-dark-muted">{$t('admin.server.uptime')}:</span> {uptime()}</div>
  <div><span class="text-dark-muted">Public URL:</span> {cfg?.public_url}</div>
  <div><span class="text-dark-muted">Max Sessions:</span> {cfg?.max_sessions}</div>
  <div><span class="text-dark-muted">Idle Timeout (min):</span> {cfg?.idle_timeout_min}</div>
  <div><span class="text-dark-muted">ICE servers:</span> {cfg?.ice_servers?.length || 0}</div>
</div>

<div class="card p-4 text-sm">
  <h2 class="font-medium mb-2">{$t('admin.server.env')}</h2>
  <pre class="text-xs bg-black/40 p-2 rounded overflow-auto max-h-64">{JSON.stringify(cfg, null, 2)}</pre>
</div>

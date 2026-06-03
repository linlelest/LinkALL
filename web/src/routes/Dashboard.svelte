<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../i18n';
  import { api } from '../lib/api';
  import { user, token } from '../lib/auth';
  import { go } from '../lib/router';

  interface Device { id: number; device_code: string; name: string; platform: string; online: boolean; last_seen: number; }
  interface Announce { id: number; title: string; content_md: string; pinned: boolean; created_at: number; }

  let devices = $state<Device[]>([]);
  let ann = $state<Announce[]>([]);
  let stats = $state<any>(null);

  onMount(async () => {
    try {
      const [ds, a, s] = await Promise.all([
        api.get<Device[]>('/api/devices'),
        api.get<Announce[]>('/api/announcements'),
        $user?.is_admin ? api.get<any>('/api/admin/stats').catch(() => null) : Promise.resolve(null)
      ]);
      devices = ds || [];
      ann = (a || []).slice(0, 5);
      stats = s;
    } catch (e) { /* ignore */ }
  });

  function fmtTime(ts: number): string {
    if (!ts) return '-';
    const d = new Date(ts * 1000);
    return d.toLocaleString();
  }
</script>

<div class="p-4 md:p-6 space-y-4">
  <div>
    <h1 class="text-xl font-semibold">{$t('dashboard.welcome')}, {$user?.username}</h1>
    <p class="text-sm text-dark-muted">{$t('app.tagline')}</p>
  </div>

  <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
    <div class="card p-3">
      <div class="text-xs text-dark-muted">{$t('dashboard.stats.devices')}</div>
      <div class="text-2xl font-bold mt-1">{devices.length}</div>
    </div>
    <div class="card p-3">
      <div class="text-xs text-dark-muted">{$t('dashboard.stats.online')}</div>
      <div class="text-2xl font-bold mt-1 text-emerald-400">{devices.filter(d => d.online).length}</div>
    </div>
    <div class="card p-3">
      <div class="text-xs text-dark-muted">{$t('dashboard.stats.sessions')}</div>
      <div class="text-2xl font-bold mt-1">{stats?.sessions ?? 0}</div>
    </div>
    <div class="card p-3">
      <div class="text-xs text-dark-muted">{$t('dashboard.stats.traffic')}</div>
      <div class="text-2xl font-bold mt-1">{(((stats?.bytes_tx||0) + (stats?.bytes_rx||0))/1024/1024).toFixed(1)} MB</div>
    </div>
  </div>

  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <div class="card p-4">
      <div class="flex items-center justify-between mb-2">
        <h2 class="font-medium">{$t('dashboard.recent.devices')}</h2>
        <button class="btn-ghost btn-sm" onclick={() => go({ name: 'devices' })}>{$t('nav.devices')}</button>
      </div>
      {#if devices.length === 0}
        <div class="text-sm text-dark-muted py-6 text-center">{$t('dashboard.empty.devices')}</div>
      {:else}
        <div class="space-y-1">
          {#each devices.slice(0, 5) as d}
            <div class="flex items-center gap-2 py-1.5 px-2 rounded hover:bg-dark-border/40">
              <span class="dot {d.online ? 'dot-online' : 'dot-offline'}"></span>
              <span class="font-mono text-xs">{d.device_code}</span>
              <span class="text-sm flex-1 truncate">{d.name || '—'}</span>
              <span class="text-[10px] text-dark-muted">{d.platform}</span>
              <button class="btn-ghost btn-sm" onclick={() => go({ name: 'control', deviceCode: d.device_code })}>→</button>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="card p-4">
      <div class="flex items-center justify-between mb-2">
        <h2 class="font-medium">{$t('dashboard.recent.announcements')}</h2>
        <button class="btn-ghost btn-sm" onclick={() => go({ name: 'announcements' })}>{$t('common.more')}</button>
      </div>
      {#if ann.length === 0}
        <div class="text-sm text-dark-muted py-6 text-center">{$t('dashboard.empty.announcements')}</div>
      {:else}
        <div class="space-y-2">
          {#each ann as a}
            <div class="text-sm">
              {#if a.pinned}<span class="text-[10px] text-amber-400 mr-1">[{$t('announce.pinned')}]</span>{/if}
              <span class="font-medium">{a.title}</span>
              <div class="text-[11px] text-dark-muted">{fmtTime(a.created_at)}</div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  <div class="flex gap-2">
    <button class="btn-primary" onclick={() => go({ name: 'connect' })}>→ {$t('dashboard.quick.connect')}</button>
    <button class="btn-ghost" onclick={() => go({ name: 'devices' })}>{$t('dashboard.quick.devices')}</button>
  </div>
</div>

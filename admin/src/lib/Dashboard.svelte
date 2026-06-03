<script lang="ts">
  import { api } from '../lib/api.svelte';
  import { formatBytes, badgeClass, fmtTime } from '../lib/format';
  import { onMount } from 'svelte';

  let stats = $state<any>(null);
  let crashStats = $state<any>(null);
  let loading = $state(true);
  let err = $state<string | null>(null);

  onMount(async () => {
    try {
      [stats, crashStats] = await Promise.all([api.stats(), api.crashStats()]);
    } catch (e: any) {
      err = e?.body?.error || e?.message || String(e);
    } finally {
      loading = false;
    }
  });
</script>

<h2 class="text-2xl font-bold mb-4">概览</h2>
{#if err}
  <p class="text-rose-400 mb-3">{err}</p>
{/if}
{#if loading}
  <p class="text-slate-400">加载中…</p>
{:else}
  <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
    <div class="card"><div class="text-xs text-slate-400">用户</div><div class="text-2xl">{stats?.users ?? '-'}</div></div>
    <div class="card"><div class="text-xs text-slate-400">设备</div><div class="text-2xl">{stats?.devices ?? '-'}</div></div>
    <div class="card"><div class="text-xs text-slate-400">活跃会话</div><div class="text-2xl">{stats?.sessions ?? '-'}</div></div>
    <div class="card"><div class="text-xs text-slate-400">崩溃（24h）</div><div class="text-2xl text-rose-300">{crashStats?.total ?? 0}</div></div>
    <div class="card"><div class="text-xs text-slate-400">总流量 Tx</div><div class="text-2xl">{formatBytes(stats?.bytes_tx)}</div></div>
    <div class="card"><div class="text-xs text-slate-400">总流量 Rx</div><div class="text-2xl">{formatBytes(stats?.bytes_rx)}</div></div>
    <div class="card"><div class="text-xs text-slate-400">公告</div><div class="text-2xl">{stats?.announcements ?? '-'}</div></div>
    <div class="card"><div class="text-xs text-slate-400">OTA 包</div><div class="text-2xl">{stats?.ota_packages ?? '-'}</div></div>
  </div>

  <h3 class="text-lg font-semibold mt-6 mb-2">最近 24h 崩溃分布</h3>
  <div class="card">
    {#if crashStats?.by_level}
      <div class="flex flex-wrap gap-2">
        {#each Object.entries(crashStats.by_level) as [lvl, n]}
          <span class="badge {badgeClass(lvl)}">{lvl}：{n}</span>
        {/each}
      </div>
    {:else}
      <p class="text-slate-500 text-sm">无</p>
    {/if}
  </div>
  <p class="text-xs text-slate-500 mt-3">最后更新：{fmtTime(Date.now() / 1000)}</p>
{/if}

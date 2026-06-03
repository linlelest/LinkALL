<script lang="ts">
  import { api, type CrashLog } from '../lib/api.svelte';
  import { badgeClass, fmtLevel, fmtTime } from '../lib/format';
  import { onMount } from 'svelte';

  let logs = $state<CrashLog[]>([]);
  let loading = $state(true);
  let err = $state<string | null>(null);
  let total = $state(0);

  let filterLevel = $state('');
  let filterDevice = $state('');
  let limit = $state(200);
  let expanded = $state<Record<string, boolean>>({});

  async function load() {
    loading = true;
    err = null;
    try {
      const r = await api.crashLogs({
        level: filterLevel || undefined,
        device_code: filterDevice || undefined,
        limit,
      });
      logs = r.logs;
      total = logs.length;
    } catch (e: any) {
      err = e?.body?.error || e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  onMount(load);
</script>

<h2 class="text-2xl font-bold mb-4">崩溃 / 日志</h2>

<div class="card mb-3 flex flex-wrap gap-3 items-end">
  <div>
    <span class="label">Level</span>
    <select class="input" bind:value={filterLevel}>
      <option value="">全部</option>
      <option value="fatal">fatal</option>
      <option value="error">error</option>
      <option value="warn">warn</option>
      <option value="info">info</option>
      <option value="debug">debug</option>
      <option value="trace">trace</option>
    </select>
  </div>
  <div>
    <span class="label">设备码</span>
    <input class="input" bind:value={filterDevice} placeholder="(任意)" />
  </div>
  <div>
    <span class="label">Limit</span>
    <input class="input w-24" type="number" min="1" max="1000" bind:value={limit} />
  </div>
  <button class="btn btn-primary" onclick={load} disabled={loading}>
    {loading ? '查询中…' : '查询'}
  </button>
  <span class="text-xs text-slate-400 ml-auto">共 {total} 条</span>
</div>

{#if err}<p class="text-rose-400 mb-3">{err}</p>{/if}

<div class="card overflow-x-auto p-0">
  <table class="w-full text-sm">
    <thead class="bg-slate-800 text-slate-300">
      <tr>
        <th class="px-2 py-2 text-left">时间</th>
        <th class="px-2 py-2 text-left">级别</th>
        <th class="px-2 py-2 text-left">设备</th>
        <th class="px-2 py-2 text-left">来源</th>
        <th class="px-2 py-2 text-left">消息</th>
        <th class="px-2 py-2 text-left">版本</th>
        <th class="px-2 py-2 text-left">IP</th>
        <th class="px-2 py-2"></th>
      </tr>
    </thead>
    <tbody>
      {#each logs as l}
        <tr class="border-t border-slate-800 hover:bg-slate-800/40">
          <td class="px-2 py-1.5 text-slate-400 whitespace-nowrap">{fmtTime(l.created_at)}</td>
          <td class="px-2 py-1.5"><span class="badge {badgeClass(l.level)}">{l.level}</span></td>
          <td class="px-2 py-1.5 font-mono text-xs">{l.device_code || '-'}</td>
          <td class="px-2 py-1.5 text-slate-300">{l.source || '-'}</td>
          <td class="px-2 py-1.5 text-slate-200 max-w-md truncate" title={l.message}>{l.message}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs">{l.app_version}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs">{l.client_ip}</td>
          <td class="px-2 py-1.5">
            <button class="text-sky-400 text-xs hover:underline"
                    onclick={() => (expanded[l.id] = !expanded[l.id])}>
              {expanded[l.id] ? '收起' : '展开'}
            </button>
          </td>
        </tr>
        {#if expanded[l.id]}
          <tr class="bg-slate-900/60">
            <td colspan="8" class="px-3 py-2 text-xs">
              <div class="text-slate-300 mb-1"><b>Stack:</b></div>
              <pre class="bg-slate-950 p-2 rounded text-slate-400 overflow-x-auto max-h-60">{l.stack || '(empty)'}</pre>
              {#if l.extra}
                <div class="text-slate-300 mt-2 mb-1"><b>Extra:</b></div>
                <pre class="bg-slate-950 p-2 rounded text-slate-400 overflow-x-auto">{l.extra}</pre>
              {/if}
              <div class="text-slate-500 mt-2">OS: {l.os_version} · 平台: {l.platform}</div>
            </td>
          </tr>
        {/if}
      {/each}
      {#if logs.length === 0 && !loading}
        <tr><td colspan="8" class="px-3 py-6 text-center text-slate-500">无数据</td></tr>
      {/if}
    </tbody>
  </table>
</div>

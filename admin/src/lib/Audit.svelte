<script lang="ts">
  import { api } from '../lib/api.svelte';
  import { fmtTime } from '../lib/format';
  import { onMount } from 'svelte';

  let logs = $state<any[]>([]);
  let loading = $state(true);
  let err = $state<string | null>(null);

  async function load() {
    loading = true; err = null;
    try { const r = await api.auditLogs(200); logs = r.logs; }
    catch (e: any) { err = e?.body?.error || e?.message || String(e); }
    finally { loading = false; }
  }
  onMount(load);
</script>

<h2 class="text-2xl font-bold mb-4">审计日志</h2>

<div class="card mb-3 flex gap-3">
  <button class="btn btn-ghost" onclick={load} disabled={loading}>刷新</button>
  <span class="text-xs text-slate-400 self-center">共 {logs.length} 条</span>
</div>

{#if err}<p class="text-rose-400 mb-3">{err}</p>{/if}

<div class="card overflow-x-auto p-0">
  <table class="w-full text-sm">
    <thead class="bg-slate-800 text-slate-300">
      <tr>
        <th class="px-2 py-2 text-left">时间</th>
        <th class="px-2 py-2 text-left">行为</th>
        <th class="px-2 py-2 text-left">操作者</th>
        <th class="px-2 py-2 text-left">对象</th>
        <th class="px-2 py-2 text-left">IP</th>
        <th class="px-2 py-2 text-left">详情</th>
      </tr>
    </thead>
    <tbody>
      {#each logs as l}
        <tr class="border-t border-slate-800 hover:bg-slate-800/40">
          <td class="px-2 py-1.5 text-slate-400 text-xs whitespace-nowrap">{fmtTime(l.created_at)}</td>
          <td class="px-2 py-1.5"><span class="badge bg-sky-900/60 text-sky-200">{l.action}</span></td>
          <td class="px-2 py-1.5 text-slate-300">{l.actor || '-'}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs font-mono">{l.target || '-'}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs">{l.client_ip}</td>
          <td class="px-2 py-1.5 text-slate-300 text-xs max-w-md truncate" title={l.detail}>{l.detail || ''}</td>
        </tr>
      {/each}
      {#if logs.length === 0 && !loading}
        <tr><td colspan="6" class="px-3 py-6 text-center text-slate-500">无审计记录</td></tr>
      {/if}
    </tbody>
  </table>
</div>

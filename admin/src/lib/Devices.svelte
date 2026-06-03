<script lang="ts">
  import { api } from '../lib/api.svelte';
  import { fmtTime } from '../lib/format';
  import { onMount } from 'svelte';

  let devices = $state<any[]>([]);
  let loading = $state(true);
  let err = $state<string | null>(null);
  let wakeResult = $state<Record<string, any>>({});
  let wakeBusy = $state<Record<string, boolean>>({});

  async function load() {
    loading = true; err = null;
    try {
      const r = await api.devices();
      devices = r.devices;
    } catch (e: any) {
      err = e?.body?.error || e?.message || String(e);
    } finally { loading = false; }
  }
  onMount(load);

  async function wake(d: any, method: string) {
    wakeBusy[d.id + method] = true;
    try {
      const r = await api.wake(d.id, method);
      wakeResult[d.id] = r;
    } catch (e: any) {
      wakeResult[d.id] = { error: e?.body?.error || e?.message || String(e) };
    } finally {
      wakeBusy[d.id + method] = false;
    }
  }

  async function revoke(d: any) {
    if (!confirm(`确认删除设备 ${d.device_code}？`)) return;
    try { await api.revokeDevice(d.id); await load(); }
    catch (e: any) { err = e?.body?.error || e?.message || String(e); }
  }
</script>

<h2 class="text-2xl font-bold mb-4">设备</h2>

<div class="card mb-3 flex gap-3">
  <button class="btn btn-ghost" onclick={load} disabled={loading}>刷新</button>
  <span class="text-xs text-slate-400 self-center">共 {devices.length} 台</span>
</div>

{#if err}<p class="text-rose-400 mb-3">{err}</p>{/if}

<div class="card overflow-x-auto p-0">
  <table class="w-full text-sm">
    <thead class="bg-slate-800 text-slate-300">
      <tr>
        <th class="px-2 py-2 text-left">设备码</th>
        <th class="px-2 py-2 text-left">名称</th>
        <th class="px-2 py-2 text-left">平台</th>
        <th class="px-2 py-2 text-left">最后在线</th>
        <th class="px-2 py-2 text-left">状态</th>
        <th class="px-2 py-2 text-left">操作</th>
      </tr>
    </thead>
    <tbody>
      {#each devices as d}
        <tr class="border-t border-slate-800 hover:bg-slate-800/40">
          <td class="px-2 py-1.5 font-mono text-xs">{d.device_code}</td>
          <td class="px-2 py-1.5">{d.name || '-'}</td>
          <td class="px-2 py-1.5 text-slate-400">{d.platform || '-'}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs">{fmtTime(d.last_seen)}</td>
          <td class="px-2 py-1.5">
            {#if d.online}
              <span class="badge bg-emerald-900/60 text-emerald-200">在线</span>
            {:else}
              <span class="badge bg-slate-800 text-slate-400">离线</span>
            {/if}
          </td>
          <td class="px-2 py-1.5 space-x-1">
            <button class="btn btn-ghost" disabled={wakeBusy[d.id + 'both']} onclick={() => wake(d, 'both')}>
              {wakeBusy[d.id + 'both'] ? '唤醒中…' : '唤醒'}
            </button>
            <button class="btn btn-ghost" disabled={wakeBusy[d.id + 'wol']} onclick={() => wake(d, 'wol')} title="仅 WoL">WoL</button>
            <button class="btn btn-ghost" disabled={wakeBusy[d.id + 'fcm']} onclick={() => wake(d, 'fcm')} title="仅 FCM">FCM</button>
            <button class="btn btn-danger" onclick={() => revoke(d)}>删除</button>
          </td>
        </tr>
        {#if wakeResult[d.id]}
          <tr class="bg-slate-900/60">
            <td colspan="6" class="px-3 py-2 text-xs text-slate-300">
              <b>唤醒结果：</b>
              <pre class="bg-slate-950 p-2 rounded text-slate-400 overflow-x-auto mt-1">{JSON.stringify(wakeResult[d.id], null, 2)}</pre>
            </td>
          </tr>
        {/if}
      {/each}
      {#if devices.length === 0 && !loading}
        <tr><td colspan="6" class="px-3 py-6 text-center text-slate-500">无设备</td></tr>
      {/if}
    </tbody>
  </table>
</div>

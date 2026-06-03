<script lang="ts">
  import { api, type FcmToken } from '../lib/api.svelte';
  import { fmtTime } from '../lib/format';
  import { onMount } from 'svelte';

  let tokens = $state<FcmToken[]>([]);
  let loading = $state(true);
  let err = $state<string | null>(null);

  async function load() {
    loading = true; err = null;
    try { const r = await api.fcmTokens(200); tokens = r.tokens; }
    catch (e: any) { err = e?.body?.error || e?.message || String(e); }
    finally { loading = false; }
  }
  onMount(load);
</script>

<h2 class="text-2xl font-bold mb-4">FCM 推送令牌</h2>

<div class="card mb-3 flex gap-3">
  <button class="btn btn-ghost" onclick={load} disabled={loading}>刷新</button>
  <span class="text-xs text-slate-400 self-center">共 {tokens.length} 条（有效 {tokens.filter(t => !t.revoked).length}）</span>
</div>

{#if err}<p class="text-rose-400 mb-3">{err}</p>{/if}

<div class="card overflow-x-auto p-0">
  <table class="w-full text-sm">
    <thead class="bg-slate-800 text-slate-300">
      <tr>
        <th class="px-2 py-2 text-left">设备码</th>
        <th class="px-2 py-2 text-left">平台</th>
        <th class="px-2 py-2 text-left">App 版本</th>
        <th class="px-2 py-2 text-left">最后心跳</th>
        <th class="px-2 py-2 text-left">状态</th>
      </tr>
    </thead>
    <tbody>
      {#each tokens as t}
        <tr class="border-t border-slate-800 hover:bg-slate-800/40">
          <td class="px-2 py-1.5 font-mono text-xs">{t.device_code}</td>
          <td class="px-2 py-1.5 text-slate-400">{t.platform}</td>
          <td class="px-2 py-1.5 text-slate-400">{t.app_version}</td>
          <td class="px-2 py-1.5 text-slate-400 text-xs">{fmtTime(t.last_seen)}</td>
          <td class="px-2 py-1.5">
            {#if t.revoked}
              <span class="badge bg-slate-800 text-slate-500">已撤销</span>
            {:else}
              <span class="badge bg-emerald-900/60 text-emerald-200">有效</span>
            {/if}
          </td>
        </tr>
      {/each}
      {#if tokens.length === 0 && !loading}
        <tr><td colspan="5" class="px-3 py-6 text-center text-slate-500">无 FCM token。Android 端需要 google-services.json + 启动推送才能注册</td></tr>
      {/if}
    </tbody>
  </table>
</div>

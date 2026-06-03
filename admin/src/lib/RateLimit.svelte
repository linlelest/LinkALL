<script lang="ts">
  import { api } from '../lib/api.svelte';
  import { onMount } from 'svelte';

  let cfg = $state<{ strictness: string; overrides: Record<string, any> } | null>(null);
  let busy = $state(false);
  let err = $state<string | null>(null);

  const levels = ['loose', 'medium', 'strict'] as const;
  let pending = $state<string>('medium');

  async function load() {
    try { cfg = await api.getRateLimit(); pending = cfg.strictness; }
    catch (e: any) { err = e?.body?.error || e?.message || String(e); }
  }
  onMount(load);

  async function save() {
    busy = true; err = null;
    try { await api.setRateLimit(pending); await load(); }
    catch (e: any) { err = e?.body?.error || e?.message || String(e); }
    finally { busy = false; }
  }
</script>

<h2 class="text-2xl font-bold mb-4">限流</h2>

<div class="card max-w-lg space-y-3">
  <p class="text-sm text-slate-400">选档后点保存立即生效（不需要重启）。每档的具体阈值见服务端配置。</p>
  <div class="flex gap-2">
    {#each levels as lv}
      <label class="flex-1 cursor-pointer">
        <input type="radio" bind:group={pending} value={lv} class="sr-only peer" />
        <div class="px-3 py-3 border border-slate-700 rounded text-center text-sm transition
                    peer-checked:bg-sky-600/30 peer-checked:border-sky-500 peer-checked:text-sky-100
                    hover:bg-slate-800">
          <div class="font-semibold capitalize">{lv}</div>
          <div class="text-xs text-slate-400 mt-1">
            {lv === 'loose' && '开发调试用，限流阈值较松'}
            {lv === 'medium' && '默认，平衡体验与安全'}
            {lv === 'strict' && '生产暴露在公网时建议'}
          </div>
        </div>
      </label>
    {/each}
  </div>
  <button class="btn btn-primary" onclick={save} disabled={busy}>{busy ? '保存中…' : '保存'}</button>
  {#if err}<p class="text-rose-400 text-sm">{err}</p>{/if}

  {#if cfg?.overrides}
    <h3 class="text-sm font-semibold mt-4">单项覆盖</h3>
    <pre class="bg-slate-950 p-2 rounded text-slate-400 text-xs overflow-x-auto">{JSON.stringify(cfg.overrides, null, 2)}</pre>
  {/if}
</div>

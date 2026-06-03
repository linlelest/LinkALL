<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../i18n';
  import { api } from '../lib/api';
  import Alert from '../lib/components/Alert.svelte';

  interface Announce { id: number; title: string; content_md: string; pinned: boolean; force_read: boolean; created_at: number; }

  let items = $state<Announce[]>([]);
  let selected = $state<Announce | null>(null);
  let err = $state('');

  onMount(async () => {
    try { items = await api.get<Announce[]>('/api/announcements'); } catch (e: any) { err = e.message; }
  });

  async function open(a: Announce) {
    selected = a;
    try { await api.post(`/api/announcements/${a.id}/read`); } catch {}
  }
  function fmt(ts: number) { return new Date(ts * 1000).toLocaleString(); }
  function render(md: string): string {
    let h = md.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    h = h.replace(/```([\s\S]*?)```/g, '<pre class="bg-black/40 rounded p-2 overflow-auto"><code>$1</code></pre>');
    h = h.replace(/`([^`]+)`/g, '<code class="bg-black/30 px-1 rounded">$1</code>');
    h = h.replace(/^### (.*)$/gm, '<h3 class="text-base font-semibold mt-3">$1</h3>');
    h = h.replace(/^## (.*)$/gm, '<h2 class="text-lg font-semibold mt-3">$1</h2>');
    h = h.replace(/^# (.*)$/gm, '<h1 class="text-xl font-semibold mt-3">$1</h1>');
    h = h.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    h = h.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    h = h.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, text, url) => {
      url = url.replace(/"/g, '&quot;');
      if (/^(javascript|data|vbscript):/i.test(url.trim())) return text;
      return `<a class="text-primary-400 underline" href="${url}" target="_blank" rel="noopener noreferrer">${text}</a>`;
    });
    h = h.replace(/\n/g, '<br/>');
    return h;
  }
</script>

<div class="p-4 md:p-6">
  <h1 class="text-xl font-semibold mb-4">{$t('announce.title')}</h1>
  {#if err}<Alert variant="error" message={err} />{/if}
  <div class="grid grid-cols-1 md:grid-cols-[300px_1fr] gap-4">
    <div class="card divide-y divide-dark-border/60">
      {#each items as a}
        <button class="w-full text-left p-3 hover:bg-dark-border/30 {selected?.id === a.id ? 'bg-primary-500/10' : ''}" onclick={() => open(a)}>
          <div class="flex items-center gap-2">
            {#if a.pinned}<span class="text-[10px] text-amber-400">[{$t('announce.pinned')}]</span>{/if}
            <span class="text-sm font-medium flex-1 truncate">{a.title}</span>
          </div>
          <div class="text-[10px] text-dark-muted mt-1">{fmt(a.created_at)}</div>
        </button>
      {/each}
      {#if items.length === 0}
        <div class="p-6 text-center text-sm text-dark-muted">{$t('announce.empty')}</div>
      {/if}
    </div>
    <div class="card p-4 min-h-[300px]">
      {#if selected}
        <h2 class="text-lg font-semibold">{selected.title}</h2>
        <div class="text-[10px] text-dark-muted mb-3">{fmt(selected.created_at)}</div>
        <div class="prose prose-invert max-w-none text-sm">{@html render(selected.content_md)}</div>
        {#if selected.force_read}
          <div class="mt-4 text-[11px] text-amber-300">[{$t('announce.force')}]</div>
        {/if}
      {:else}
        <div class="text-dark-muted text-sm text-center py-12">{$t('common.more')}</div>
      {/if}
    </div>
  </div>
</div>

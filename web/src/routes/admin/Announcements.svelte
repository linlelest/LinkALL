<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../../i18n';
  import { api } from '../../lib/api';
  import Alert from '../../lib/components/Alert.svelte';
  import Modal from '../../lib/components/Modal.svelte';

  interface Announce { id: number; title: string; content_md: string; platform: string; min_version: string; pinned: boolean; force_read: boolean; signature: string; created_at: number; updated_at: number; revoked: boolean; }

  let items = $state<Announce[]>([]);
  let editing = $state<Announce | null>(null);
  let err = $state('');

  onMount(async () => {
    try { items = await api.get<Announce[]>('/api/announcements?include_revoked=true'); } catch (e: any) { err = e.message; }
  });

  function newAnn() {
    editing = { id: 0, title: '', content_md: '', platform: '', min_version: '', pinned: false, force_read: false, signature: '', created_at: 0, updated_at: 0, revoked: false };
  }
  async function save() {
    if (!editing) return;
    try {
      if (editing.id === 0) {
        const r = await api.post<Announce>('/api/announcements', editing);
        items = [r, ...items];
      } else {
        await api.patch(`/api/announcements/${editing.id}`, editing);
        items = items.map(i => i.id === editing!.id ? editing! : i);
      }
      editing = null;
    } catch (e: any) { err = e.message; }
  }
  async function del(a: Announce) {
    if (!confirm('Delete announcement "' + a.title + '"?')) return;
    try { await api.del(`/api/announcements/${a.id}`); items = items.map(i => i.id === a.id ? { ...i, revoked: true } : i); } catch (e: any) { err = e.message; }
  }
  function fmt(ts: number) { return ts ? new Date(ts * 1000).toLocaleString() : '-'; }
</script>

<div class="flex items-center justify-between mb-4">
  <h1 class="text-xl font-semibold">{$t('admin.ann.title')}</h1>
  <button class="btn-primary" onclick={newAnn}>＋ {$t('admin.ann.new')}</button>
</div>
{#if err}<Alert variant="error" message={err} />{/if}

<div class="card overflow-hidden">
  <table class="w-full text-sm">
    <thead class="bg-dark-border/40 text-xs text-dark-muted">
      <tr>
        <th class="text-left px-3 py-2">{$t('announce.title')}</th>
        <th class="text-left px-3 py-2">{$t('admin.ann.platform')}</th>
        <th class="text-left px-3 py-2">{$t('announce.pinned')}</th>
        <th class="text-left px-3 py-2">{$t('announce.force')}</th>
        <th class="text-left px-3 py-2">{$t('common.refresh')}</th>
        <th class="text-right px-3 py-2">{$t('common.more')}</th>
      </tr>
    </thead>
    <tbody>
      {#each items as a}
        <tr class="border-t border-dark-border/60 {a.revoked ? 'opacity-50' : ''}">
          <td class="px-3 py-2">{a.title}</td>
          <td class="px-3 py-2 text-xs">{a.platform || '*'}</td>
          <td class="px-3 py-2">{a.pinned ? '★' : ''}</td>
          <td class="px-3 py-2">{a.force_read ? '✓' : ''}</td>
          <td class="px-3 py-2 text-[11px] text-dark-muted">{fmt(a.updated_at)}</td>
          <td class="px-3 py-2 text-right">
            <button class="btn-ghost btn-sm" onclick={() => (editing = { ...a })}>{$t('common.edit')}</button>
            {#if !a.revoked}<button class="btn-danger btn-sm" onclick={() => del(a)}>{$t('common.delete')}</button>{/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<Modal bind:open={editing} title="admin.ann.new">
  {#if editing}
    <div class="space-y-2">
      <div>
        <label class="label">{$t('announce.title')}</label>
        <input class="input" bind:value={editing.title} />
      </div>
      <div>
        <label class="label">{$t('admin.ann.content')}</label>
        <textarea class="input h-40 font-mono text-xs" bind:value={editing.content_md}></textarea>
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div>
          <label class="label">{$t('admin.ann.platform')}</label>
          <input class="input" bind:value={editing.platform} placeholder="win64 / android-arm64 / 空" />
        </div>
        <div>
          <label class="label">{$t('admin.ann.min_version')}</label>
          <input class="input" bind:value={editing.min_version} placeholder="1.0.0" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-2">
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editing.pinned} /> {$t('admin.ann.pinned')}</label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editing.force_read} /> {$t('admin.ann.force')}</label>
      </div>
      <div>
        <label class="label">{$t('admin.ann.signature')}</label>
        <input class="input font-mono text-xs" bind:value={editing.signature} placeholder="ed25519:..." />
      </div>
    </div>
  {/if}
  {#snippet actions()}
    <button class="btn-ghost" onclick={() => (editing = null)}>{$t('common.cancel')}</button>
    <button class="btn-primary" onclick={save}>{$t('common.save')}</button>
  {/snippet}
</Modal>

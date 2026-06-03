<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../../i18n';
  import { api } from '../../lib/api';
  import Alert from '../../lib/components/Alert.svelte';

  interface Invite { id: number; code: string; max_uses: number; used_count: number; ttl_hours: number; expires_at: number; revoked: boolean; note: string; }
  let items = $state<Invite[]>([]);
  let max = $state(1);
  let ttl = $state(72);
  let note = $state('');
  let err = $state('');

  onMount(async () => {
    try { items = await api.get<Invite[]>('/api/invites'); } catch (e: any) { err = e.message; }
  });

  async function create() {
    err = '';
    try {
      const inv = await api.post<Invite>('/api/invites', { max_uses: max, ttl_hours: ttl, note });
      items = [inv, ...items];
    } catch (e: any) { err = e.message; }
  }
  async function revoke(id: number) {
    try { await api.del(`/api/invites/${id}`); items = items.map(i => i.id === id ? { ...i, revoked: true } : i); } catch (e: any) { err = e.message; }
  }
  function fmt(ts: number) { return new Date(ts * 1000).toLocaleString(); }
  function copy(text: string) { navigator.clipboard?.writeText(text); }
</script>

<h1 class="text-xl font-semibold mb-4">{$t('admin.invites.title')}</h1>
{#if err}<Alert variant="error" message={err} />{/if}

<div class="card p-3 mb-3 flex flex-wrap items-end gap-2">
  <div>
    <label class="label">{$t('admin.invites.max')}</label>
    <input class="input w-24" type="number" bind:value={max} min="1" max="100" />
  </div>
  <div>
    <label class="label">{$t('admin.invites.ttl')}</label>
    <input class="input w-24" type="number" bind:value={ttl} min="1" max="8760" />
  </div>
  <div class="flex-1 min-w-[200px]">
    <label class="label">{$t('admin.invites.note')}</label>
    <input class="input" bind:value={note} />
  </div>
  <button class="btn-primary" onclick={create}>＋ {$t('common.add')}</button>
</div>

<div class="card overflow-hidden">
  <table class="w-full text-sm">
    <thead class="bg-dark-border/40 text-xs text-dark-muted">
      <tr>
        <th class="text-left px-3 py-2">{$t('admin.invites.code')}</th>
        <th class="text-left px-3 py-2">{$t('admin.invites.uses')}</th>
        <th class="text-left px-3 py-2">{$t('admin.invites.expires')}</th>
        <th class="text-left px-3 py-2">{$t('admin.invites.note')}</th>
        <th class="text-right px-3 py-2">{$t('common.more')}</th>
      </tr>
    </thead>
    <tbody>
      {#each items as i}
        <tr class="border-t border-dark-border/60">
          <td class="px-3 py-2 font-mono">{i.code} {#if i.revoked}<span class="text-rose-400 text-[10px]">[{$t('common.revoke')}]</span>{/if}</td>
          <td class="px-3 py-2">{i.used_count}/{i.max_uses}</td>
          <td class="px-3 py-2 text-[11px] text-dark-muted">{fmt(i.expires_at)}</td>
          <td class="px-3 py-2 text-xs">{i.note}</td>
          <td class="px-3 py-2 text-right">
            <button class="btn-ghost btn-sm" onclick={() => copy(i.code)}>{$t('common.copy')}</button>
            {#if !i.revoked}<button class="btn-danger btn-sm" onclick={() => revoke(i.id)}>{$t('common.revoke')}</button>{/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

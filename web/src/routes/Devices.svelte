<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { t } from '../i18n';
  import { api, ApiError } from '../lib/api';
  import { go } from '../lib/router';
  import Modal from '../lib/components/Modal.svelte';
  import Alert from '../lib/components/Alert.svelte';
  import Select from '../lib/components/Select.svelte';

  interface Device {
    id: number; device_code: string; name: string; platform: string;
    os_version: string; app_version: string; allow_anonymous: boolean;
    require_device_code: boolean; accept_connections: boolean;
    online: boolean; last_ip: string; last_seen: number; tag: string; notes: string;
  }

  let devices = $state<Device[]>([]);
  let err = $state('');
  let editing = $state<Device | null>(null);
  let showAdd = $state(false);
  let showReset = $state<Device | null>(null);
  let newDev = $state({ device_code: '', device_password: '', name: '', platform: 'win64' });
  let resetVals = $state({ new_code: '', new_password: '' });
  let collapsedTags = $state<Set<string>>(new Set());
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  const platforms = [
    { label: 'Windows', value: 'win64' },
    { label: 'Linux', value: 'linux-x86_64' },
    { label: 'Android', value: 'android-arm64' },
    { label: 'Web', value: 'web' }
  ];

  onMount(async () => {
    try { devices = await api.get<Device[]>('/api/devices'); } catch (e: any) { err = e.message; }
    refreshTimer = setInterval(() => {
      api.get<Device[]>('/api/devices').then(d => devices = d).catch(() => {});
    }, 5000);
  });

  onDestroy(() => { if (refreshTimer) clearInterval(refreshTimer); });

  async function reload() { try { devices = await api.get<Device[]>('/api/devices'); } catch (e: any) { err = e.message; } }

  async function add() {
    err = '';
    try {
      const r = await api.post<any>('/api/devices/register', { ...newDev });
      showAdd = false;
      newDev = { device_code: '', device_password: '', name: '', platform: 'win64' };
      await reload();
    } catch (e: any) { err = e.message; }
  }

  async function saveEdit() {
    if (!editing) return;
    err = '';
    try {
      await api.patch<any>(`/api/devices/${editing.id}`, {
        name: editing.name, allow_anonymous: editing.allow_anonymous,
        require_device_code: editing.require_device_code,
        accept_connections: editing.accept_connections,
        tag: editing.tag, notes: editing.notes
      });
      editing = null;
      await reload();
    } catch (e: any) { err = e.message; }
  }

  async function doReset() {
    if (!showReset) return;
    err = '';
    try {
      await api.post<any>(`/api/devices/${showReset.id}/reset-code`, resetVals);
      showReset = null;
      resetVals = { new_code: '', new_password: '' };
      await reload();
    } catch (e: any) { err = e.message; }
  }

  async function del(d: Device) {
    if (!confirm($t('devices.confirm.delete'))) return;
    try { await api.del(`/api/devices/${d.id}`); await reload(); } catch (e: any) { err = e.message; }
  }

  function copy(text: string) {
    navigator.clipboard?.writeText(text);
  }

  function fmtTime(ts: number) { return ts ? new Date(ts * 1000).toLocaleString() : '-'; }

  let groups = $derived.by(() => {
    const map = new Map<string, Device[]>();
    for (const d of devices) {
      const key = d.tag || '(default)';
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(d);
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  });

  function toggleTag(tag: string) {
    const next = new Set(collapsedTags);
    if (next.has(tag)) next.delete(tag); else next.add(tag);
    collapsedTags = next;
  }
</script>

<div class="p-4 md:p-6 space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-semibold">{$t('devices.title')}</h1>
    <div class="flex items-center gap-2">
      <span class="text-[11px] text-dark-muted">{$t('common.refresh')} 5s</span>
      <button class="btn-primary" onclick={() => (showAdd = true)}>＋ {$t('devices.new')}</button>
    </div>
  </div>

  {#if err}<Alert variant="error" message={err} />{/if}

  <div class="space-y-3">
    {#each groups as [tag, items]}
      <div class="card overflow-hidden">
        <button class="w-full flex items-center justify-between px-3 py-2 text-sm font-medium bg-dark-border/20 hover:bg-dark-border/40"
          onclick={() => toggleTag(tag)}>
          <span>{tag} <span class="text-dark-muted text-[11px]">({items.length})</span></span>
          <span class="text-dark-muted">{collapsedTags.has(tag) ? '▶' : '▼'}</span>
        </button>
        {#if !collapsedTags.has(tag)}
          <table class="w-full text-sm">
            <thead class="bg-dark-border/40 text-xs text-dark-muted">
              <tr>
                <th class="text-left px-3 py-2">{$t('devices.code')}</th>
                <th class="text-left px-3 py-2">{$t('devices.name')}</th>
                <th class="text-left px-3 py-2">{$t('devices.platform')}</th>
                <th class="text-left px-3 py-2">{$t('common.settings')}</th>
                <th class="text-left px-3 py-2">{$t('devices.last_seen')}</th>
                <th class="text-right px-3 py-2">{$t('common.more')}</th>
              </tr>
            </thead>
            <tbody>
              {#each items as d}
                <tr class="border-t border-dark-border/60 hover:bg-dark-border/20">
                  <td class="px-3 py-2 font-mono text-xs">
                    <span class="dot {d.online ? 'dot-online' : 'dot-offline'} mr-2"></span>
                    {d.device_code}
                    <button class="text-dark-muted ml-1" onclick={() => copy(d.device_code)} title={$t('devices.actions.copy_code')}>⧉</button>
                  </td>
                  <td class="px-3 py-2">{d.name || '—'}</td>
                  <td class="px-3 py-2 text-xs">{d.platform}</td>
                  <td class="px-3 py-2 text-[10px] space-x-1">
                    {#if d.allow_anonymous}<span class="px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-300">anon</span>{/if}
                    {#if d.require_device_code}<span class="px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-300">code</span>{/if}
                    {#if !d.accept_connections}<span class="px-1.5 py-0.5 rounded bg-rose-500/15 text-rose-300">paused</span>{/if}
                  </td>
                  <td class="px-3 py-2 text-[11px] text-dark-muted">{fmtTime(d.last_seen)}</td>
                  <td class="px-3 py-2 text-right space-x-1">
                    <button class="btn-ghost btn-sm" onclick={() => go({ name: 'control', deviceCode: d.device_code })}>{$t('devices.actions.connect')}</button>
                    <button class="btn-ghost btn-sm" onclick={() => (editing = { ...d })}>{$t('devices.actions.edit')}</button>
                    <button class="btn-ghost btn-sm" onclick={() => { showReset = d; resetVals = { new_code: '', new_password: '' }; }}>{$t('devices.reset')}</button>
                    <button class="btn-danger btn-sm" onclick={() => del(d)}>{$t('common.delete')}</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    {:else}
      <div class="card p-8 text-center text-sm text-dark-muted">{$t('dashboard.empty.devices')}</div>
    {/each}
  </div>
</div>

<Modal bind:open={showAdd} title="devices.new">
  <div class="space-y-3">
    <div>
      <label class="label">{$t('devices.code')}</label>
      <input class="input font-mono uppercase" bind:value={newDev.device_code} placeholder="ABCD1234EFGH" />
      <div class="text-[10px] text-dark-muted mt-1">{$t('devices.code.hint')}</div>
    </div>
    <div>
      <label class="label">{$t('devices.password')}</label>
      <input class="input" type="password" bind:value={newDev.device_password} />
    </div>
    <div>
      <label class="label">{$t('devices.name')}</label>
      <input class="input" bind:value={newDev.name} />
    </div>
    <div>
      <label class="label">{$t('devices.platform')}</label>
      <Select bind:value={newDev.platform} {platforms} />
    </div>
  </div>
  {#snippet actions()}
    <button class="btn-ghost" onclick={() => (showAdd = false)}>{$t('common.cancel')}</button>
    <button class="btn-primary" onclick={add}>{$t('common.save')}</button>
  {/snippet}
</Modal>

<Modal bind:open={editing} title="devices.actions.edit">
  {#if editing}
    <div class="space-y-3">
      <div>
        <label class="label">{$t('devices.name')}</label>
        <input class="input" bind:value={editing.name} />
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div>
          <label class="label">{$t('devices.tag')}</label>
          <input class="input" bind:value={editing.tag} />
        </div>
        <div>
          <label class="label">{$t('devices.notes')}</label>
          <input class="input" bind:value={editing.notes} />
        </div>
      </div>
      <div class="space-y-1">
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editing.allow_anonymous} /> {$t('devices.allow_anon')}</label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editing.require_device_code} /> {$t('devices.require_code')}</label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={editing.accept_connections} /> {$t('devices.accept')}</label>
      </div>
    </div>
  {/if}
  {#snippet actions()}
    <button class="btn-ghost" onclick={() => (editing = null)}>{$t('common.cancel')}</button>
    <button class="btn-primary" onclick={saveEdit}>{$t('common.save')}</button>
  {/snippet}
</Modal>

<Modal bind:open={showReset} title="devices.reset">
  <div class="space-y-3">
    <div>
      <label class="label">{$t('devices.code')}</label>
      <input class="input font-mono uppercase" bind:value={resetVals.new_code} placeholder="留空自动生成" />
    </div>
    <div>
      <label class="label">{$t('devices.password')}</label>
      <input class="input" type="password" bind:value={resetVals.new_password} />
    </div>
  </div>
  {#snippet actions()}
    <button class="btn-ghost" onclick={() => (showReset = null)}>{$t('common.cancel')}</button>
    <button class="btn-primary" onclick={doReset}>{$t('common.save')}</button>
  {/snippet}
</Modal>

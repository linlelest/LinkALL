<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../../i18n';
  import { api } from '../../lib/api';
  import Alert from '../../lib/components/Alert.svelte';

  interface OTA { id: number; platform: string; version: string; channel: string; file_name: string; file_size: number; sha256: string; signature: string; release_notes: string; force_update: boolean; min_supported_version: string; downloads: number; created_at: number; revoked: boolean; }

  let items = $state<OTA[]>([]);
  let platform = $state('win64');
  let version = $state('');
  let channel = $state('stable');
  let forceUpdate = $state(false);
  let minVer = $state('');
  let notes = $state('');
  let signature = $state('');
  let file: File | null = $state(null);
  let err = $state('');
  let progress = $state(0);

  const platforms = [
    { label: 'Windows (win64)', value: 'win64' },
    { label: 'Linux x86_64', value: 'linux-x86_64' },
    { label: 'Android arm64', value: 'android-arm64' },
    { label: 'Web', value: 'web' }
  ];
  const channels = [
    { label: 'admin.ota.channels.stable', value: 'stable' },
    { label: 'admin.ota.channels.beta', value: 'beta' },
    { label: 'admin.ota.channels.canary', value: 'canary' }
  ];

  onMount(async () => {
    try { items = await api.get<OTA[]>('/api/ota/list?include_revoked=true'); } catch (e: any) { err = e.message; }
  });

  function onFile(e: Event) { file = (e.target as HTMLInputElement).files?.[0] || null; }

  async function upload() {
    err = ''; progress = 0;
    if (!file) { err = '请选择文件'; return; }
    if (!version) { err = '请填写版本号'; return; }
    const fd = new FormData();
    fd.append('file', file);
    fd.append('platform', platform);
    fd.append('version', version);
    fd.append('channel', channel);
    fd.append('force_update', String(forceUpdate));
    fd.append('min_supported_version', minVer);
    fd.append('release_notes', notes);
    fd.append('signature', signature);
    try {
      // 用 XHR 监听进度
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', (window.location.origin) + '/api/ota/upload');
        const tk = localStorage.getItem('linkall.token');
        if (tk) xhr.setRequestHeader('Authorization', 'Bearer ' + JSON.parse(tk).replace(/"/g, ''));
        xhr.upload.onprogress = (e) => { if (e.lengthComputable) progress = Math.round(e.loaded / e.total * 100); };
        xhr.onload = () => { if (xhr.status >= 200 && xhr.status < 300) resolve(); else reject(new Error(xhr.responseText || ('HTTP ' + xhr.status))); };
        xhr.onerror = () => reject(new Error('network'));
        xhr.send(fd);
      });
      const fresh = await api.get<OTA[]>('/api/ota/list?include_revoked=true');
      items = fresh;
      file = null; version = ''; notes = ''; signature = ''; minVer = '';
    } catch (e: any) { err = e.message; }
  }

  async function del(o: OTA) {
    if (!confirm('Delete ' + o.platform + ' v' + o.version + '?')) return;
    try { await api.del(`/api/ota/${o.id}`); items = items.map(x => x.id === o.id ? { ...x, revoked: true } : x); } catch (e: any) { err = e.message; }
  }

  function fmtSize(n: number) {
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
    if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
    return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB';
  }
  function fmtDate(ts: number) { return new Date(ts * 1000).toLocaleString(); }
</script>

<h1 class="text-xl font-semibold mb-4">{$t('admin.ota.title')}</h1>
{#if err}<Alert variant="error" message={err} />{/if}

<div class="card p-3 mb-4">
  <h2 class="font-medium mb-2">{$t('admin.ota.upload')}</h2>
  <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
    <div>
      <label class="label">{$t('admin.ota.platform')}</label>
      <select class="input" bind:value={platform}>
        {#each platforms as p}<option value={p.value}>{p.label}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label">{$t('admin.ota.version')}</label>
      <input class="input" bind:value={version} placeholder="1.0.0" />
    </div>
    <div>
      <label class="label">{$t('admin.ota.channel')}</label>
      <select class="input" bind:value={channel}>
        {#each channels as c}<option value={c.value}>{$t(c.label)}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label">{$t('admin.ota.min_ver')}</label>
      <input class="input" bind:value={minVer} placeholder="0.9.0" />
    </div>
  </div>
  <div class="mt-2">
    <label class="label">{$t('admin.ota.notes')}</label>
    <textarea class="input h-20 text-xs" bind:value={notes}></textarea>
  </div>
  <div class="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2">
    <div>
      <label class="label">{$t('admin.ota.signature')}</label>
      <input class="input font-mono text-xs" bind:value={signature} placeholder="ed25519:..." />
    </div>
    <div>
      <label class="label">{$t('common.upload')}</label>
      <input class="input" type="file" onchange={onFile} />
    </div>
  </div>
  <div class="mt-3 flex items-center gap-2">
    <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={forceUpdate} /> {$t('admin.ota.force')}</label>
    <button class="btn-primary ml-auto" onclick={upload}>{$t('common.upload')}</button>
  </div>
  {#if progress > 0 && progress < 100}
    <div class="mt-2 h-2 bg-dark-border rounded overflow-hidden">
      <div class="h-2 bg-primary-500" style="width: {progress}%"></div>
    </div>
    <div class="text-xs text-dark-muted mt-1">{progress}%</div>
  {/if}
</div>

<div class="card overflow-hidden">
  <table class="w-full text-sm">
    <thead class="bg-dark-border/40 text-xs text-dark-muted">
      <tr>
        <th class="text-left px-3 py-2">{$t('admin.ota.platform')}</th>
        <th class="text-left px-3 py-2">{$t('admin.ota.version')}</th>
        <th class="text-left px-3 py-2">{$t('admin.ota.channel')}</th>
        <th class="text-left px-3 py-2">{$t('admin.ota.size')}</th>
        <th class="text-left px-3 py-2">{$t('admin.ota.downloads')}</th>
        <th class="text-left px-3 py-2">{$t('common.refresh')}</th>
        <th class="text-right px-3 py-2">{$t('common.more')}</th>
      </tr>
    </thead>
    <tbody>
      {#each items as o}
        <tr class="border-t border-dark-border/60 {o.revoked ? 'opacity-50' : ''}">
          <td class="px-3 py-2 text-xs">{o.platform}</td>
          <td class="px-3 py-2 font-mono">{o.version} {#if o.force_update}<span class="text-rose-400 text-[10px]">[force]</span>{/if}</td>
          <td class="px-3 py-2 text-xs">{o.channel}</td>
          <td class="px-3 py-2 text-xs">{fmtSize(o.file_size)}</td>
          <td class="px-3 py-2 text-xs">{o.downloads}</td>
          <td class="px-3 py-2 text-[11px] text-dark-muted">{fmtDate(o.created_at)}</td>
          <td class="px-3 py-2 text-right">
            {#if !o.revoked}<button class="btn-danger btn-sm" onclick={() => del(o)}>{$t('common.delete')}</button>{/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

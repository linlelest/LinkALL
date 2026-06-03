<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type DeviceInfo, type ServerConfig, type ConnectionInfo } from './lib/api';

  let dev = $state<DeviceInfo | null>(null);
  let cfg = $state<ServerConfig | null>(null);
  let server = $state('');
  let status = $state<any>({ running: false, signaling: 'offline', screen_w: 0, screen_h: 0, last_error: '' });
  let autostart = $state(false);
  let log = $state<string[]>([]);
  let pending = $state<ConnectionInfo | null>(null);

  // 注册/登录表单
  let newCode = $state('');
  let newPw = $state('');
  let loginCode = $state('');
  let loginPw = $state('');
  let showLogin = $state(true);
  let newName = $state('');
  let err = $state('');

  onMount(async () => {
    try {
      cfg = await api.getServer();
      server = cfg?.official_server || '';
      dev = await api.getDevice();
      autostart = await api.getAutostart();
      status = await api.getStatus();
    } catch (e: any) { err = e.message; }
    // 订阅事件
    try { await api.onConnectionRequest((info) => { pending = info; }); } catch {}
    try { await api.onLog((l) => { log = [...log.slice(-200), l]; }); } catch {}
    try { await api.onStatus(async () => { try { status = await api.getStatus(); } catch {} }); } catch {}
  });

  async function saveServer() {
    await api.setServer(server);
    cfg = await api.getServer();
  }
  async function register() {
    err = '';
    try {
      dev = await api.registerDevice({ device_code: newCode, device_password: newPw, name: newName, platform: 'win64' });
      showLogin = true;
    } catch (e: any) { err = e.message; }
  }
  async function login() {
    err = '';
    try {
      dev = await api.loginDevice({ device_code: loginCode, device_password: loginPw });
    } catch (e: any) { err = e.message; }
  }
  async function logout() { await api.logoutDevice(); dev = null; }
  async function start() { try { await api.startService(); status = await api.getStatus(); } catch (e: any) { err = e.message; } }
  async function stop() { try { await api.stopService(); status = await api.getStatus(); } catch (e: any) { err = e.message; } }
  async function toggleAutostart() { autostart = !autostart; await api.setAutostart(autostart); }
  async function allow(mode: 'once' | 'permanent') {
    pending = null;
    try { const { invoke } = await import('@tauri-apps/api/core'); await invoke('respond_request', { reqId: pending?.id, allowed: mode }); } catch (e: any) { err = e.message; }
  }
  async function deny() {
    pending = null;
    try { const { invoke } = await import('@tauri-apps/api/core'); await invoke('respond_request', { reqId: pending?.id, allowed: 'denied' }); } catch {}
  }
  async function updateFlags(field: 'allowAn'|'reqCode'|'accept', v: boolean) {
    if (!dev) return;
    try {
      dev = await api.updateFlags(
        field === 'allowAn' ? v : dev.allow_anonymous,
        field === 'reqCode' ? v : dev.require_device_code,
        field === 'accept' ? v : dev.accept_connections,
      );
    } catch (e: any) { err = e.message; }
  }
  async function reset() {
    const np = prompt('新的设备码 (>=6 chars)');
    if (!np) return;
    try { dev = await api.resetCode('', np); } catch (e: any) { err = e.message; }
  }
</script>

<div class="h-full overflow-auto p-4 space-y-4 max-w-2xl mx-auto">
  <header class="flex items-center justify-between">
    <div class="flex items-center gap-3">
      <div class="text-2xl font-bold text-primary-400">LinkALL</div>
      <div class="text-xs text-[#8a96a8]">被控端 · Windows</div>
    </div>
    <div class="flex items-center gap-2 text-xs">
      <span class="dot {status.running ? (status.signaling === 'online' ? 'dot-online' : 'dot-busy') : 'dot-offline'}"></span>
      <span>{status.running ? `运行中 · 信令 ${status.signaling}` : '未运行'}</span>
    </div>
  </header>

  {#if !dev}
    <div class="card space-y-3">
      <div class="flex border-b border-[#1f2733] -m-4 mb-3">
        <button class="px-3 py-2 text-sm {showLogin ? 'border-b-2 border-primary-500 text-white' : 'text-[#8a96a8]'}" onclick={() => (showLogin = true)}>登录</button>
        <button class="px-3 py-2 text-sm {!showLogin ? 'border-b-2 border-primary-500 text-white' : 'text-[#8a96a8]'}" onclick={() => (showLogin = false)}>注册</button>
      </div>
      <div>
        <label class="label">服务器地址</label>
        <input class="input" bind:value={server} placeholder="http://127.0.0.1:8080" />
        <div class="flex justify-end mt-1">
          <button class="btn-ghost btn-sm" onclick={saveServer}>保存</button>
        </div>
      </div>
      {#if showLogin}
        <div><label class="label">设备编号</label><input class="input font-mono uppercase" bind:value={loginCode} /></div>
        <div><label class="label">设备码</label><input class="input" type="password" bind:value={loginPw} /></div>
        <button class="btn-primary w-full" onclick={login}>登录</button>
      {:else}
        <div><label class="label">设备编号 (留空自动生成)</label><input class="input font-mono uppercase" bind:value={newCode} /></div>
        <div><label class="label">设备码 (>=6 字符)</label><input class="input" type="password" bind:value={newPw} /></div>
        <div><label class="label">设备名</label><input class="input" bind:value={newName} placeholder="kevin-pc" /></div>
        <button class="btn-primary w-full" onclick={register}>注册并登录</button>
      {/if}
      {#if err}<div class="text-rose-300 text-sm border border-rose-500/30 rounded px-2 py-1 bg-rose-500/10">{err}</div>{/if}
    </div>
  {:else}
    <div class="card space-y-3">
      <div class="flex items-center justify-between">
        <div>
          <div class="text-sm text-[#8a96a8]">设备</div>
          <div class="font-mono text-lg">{dev.device_code}</div>
          <div class="text-xs text-[#8a96a8]">{dev.name || '—'} · {dev.platform}</div>
        </div>
        <div class="flex flex-col gap-1">
          {#if status.running}
            <button class="btn-danger btn-sm" onclick={stop}>停止服务</button>
          {:else}
            <button class="btn-primary btn-sm" onclick={start}>启动服务</button>
          {/if}
          <button class="btn-ghost btn-sm" onclick={logout}>退出登录</button>
        </div>
      </div>
      <div class="grid grid-cols-2 gap-2 text-sm">
        <label class="flex items-center gap-2"><input type="checkbox" checked={dev.allow_anonymous} onchange={(e) => updateFlags('allowAn', (e.target as HTMLInputElement).checked)} /> 允许匿名连接</label>
        <label class="flex items-center gap-2"><input type="checkbox" checked={dev.require_device_code} onchange={(e) => updateFlags('reqCode', (e.target as HTMLInputElement).checked)} /> 需要设备码</label>
        <label class="flex items-center gap-2"><input type="checkbox" checked={dev.accept_connections} onchange={(e) => updateFlags('accept', (e.target as HTMLInputElement).checked)} /> 接受连接</label>
        <label class="flex items-center gap-2"><input type="checkbox" checked={autostart} onchange={toggleAutostart} /> 开机自启</label>
      </div>
      <div class="text-[10px] text-[#8a96a8]">屏幕 {status.screen_w}×{status.screen_h} · 错误: {status.last_error || '无'}</div>
      <div class="flex gap-2">
        <button class="btn-ghost btn-sm" onclick={reset}>重置设备码</button>
        <button class="btn-ghost btn-sm" onclick={() => api.quit()}>退出软件</button>
      </div>
    </div>
  {/if}

  <div class="card">
    <div class="text-sm font-medium mb-1">日志</div>
    <div class="bg-black/40 rounded p-2 text-[10px] font-mono h-32 overflow-auto">
      {#each log as l}<div>{l}</div>{/each}
    </div>
  </div>

  {#if pending}
    <div class="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
      <div class="card max-w-sm w-full space-y-3">
        <div class="text-lg font-medium">收到连接请求</div>
        <div class="text-sm">控制器 ID: <span class="font-mono">{pending.from}</span></div>
        <div class="text-sm">设备: <span class="font-mono">{pending.device_code}</span></div>
        <div class="text-sm">模式: {pending.mode}</div>
        <div class="text-[10px] text-[#8a96a8]">时间: {new Date(pending.ts).toLocaleString()}</div>
        <div class="flex flex-wrap gap-2 justify-end">
          <button class="btn-danger" onclick={deny}>拒绝</button>
          <button class="btn-ghost" onclick={() => allow('once')}>仅本次</button>
          <button class="btn-primary" onclick={() => allow('permanent')}>永久允许</button>
        </div>
      </div>
    </div>
  {/if}
</div>

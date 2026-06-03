<script lang="ts">
  import { onDestroy } from 'svelte';
  import { t } from '../i18n';
  import { api, apiBase, ApiError } from '../lib/api';
  import { token } from '../lib/auth';
  import { go } from '../lib/router';
  import { ControlClient } from '../lib/webrtc';
  import Alert from '../lib/components/Alert.svelte';

  interface Props { deviceCode: string; }
  let { deviceCode }: Props = $props();

  let code = $state(deviceCode || '');
  let password = $state('');
  let mode = $state<'account' | 'anonymous'>('anonymous');
  let status = $state('idle');
  let log = $state<string[]>([]);
  let stream: MediaStream | null = $state(null);
  let client: ControlClient | null = $state(null);
  let privacy = $state(false);
  let scale = $state(100);
  let bitrate = $state(4096);
  let fps = $state(30);
  let codec = $state<'h264' | 'vp9' | 'av1' | 'vp8'>('h264');
  let showKb = $state(false);
  let showTransfer = $state(false);
  let showFiles = $state(false);
  let audioMuted = $state(true);
  let serverCfg = $state<any>(null);
  let videoEl: HTMLVideoElement;
  let started = $derived(status === 'ack:once' || status === 'ack:permanent' || status === 'dc:open');
  let rtt = $state(0);
  let loss = $state(0);

  $effect(() => {
    api.get<any>('/api/config').then((c) => (serverCfg = c)).catch(() => {});
  });

  onDestroy(() => client?.close());

  function pushLog(s: string) {
    log = [...log.slice(-200), `[${new Date().toLocaleTimeString()}] ${s}`];
  }

  async function start() {
    if (!code) return;
    status = 'signaling:connecting';
    pushLog('connect -> ' + code);
    const base = apiBase();
    const wsUrl = base.replace(/^http/, 'ws') + '/ws/signaling';
    const tk = $token;
    client = new ControlClient({
      signalingUrl: wsUrl,
      deviceCode: code.trim().toUpperCase(),
      token: tk || '',
      iceServers: serverCfg?.ice_servers,
      onVideo: (s) => { stream = s; if (videoEl) { videoEl.srcObject = s; videoEl.play().catch(() => {}); } },
      onData: (d) => {
        pushLog('data: ' + d);
        try {
          const cmd = JSON.parse(d);
          if (cmd.op === 'clip' && cmd.text) {
            navigator.clipboard?.writeText(cmd.text).catch(() => {});
            pushLog($t('control.clip.recv') + ': ' + cmd.text.slice(0, 50));
          }
        } catch {}
      },
      onStatus: (s) => { status = s; pushLog('status: ' + s); },
      onLog: pushLog
    });
    try { await client.connect(); } catch (e: any) { status = 'error:' + e.message; pushLog('err: ' + e.message); }
  }

  function stop() { client?.close(); client = null; stream = null; status = 'idle'; }

  function sendConfig() {
    client?.sendConfig({ scale, bitrate_kbps: bitrate, fps, codec, privacy });
  }

  // ===== 虚拟鼠标/键盘事件 =====
  let dragState: { active: boolean; lastX: number; lastY: number; } = { active: false, lastX: 0, lastY: 0 };

  function onMouseDown(e: MouseEvent) {
    if (!videoEl) return;
    const r = videoEl.getBoundingClientRect();
    const x = ((e.clientX - r.left) / r.width) * 100;
    const y = ((e.clientY - r.top) / r.height) * 100;
    dragState = { active: true, lastX: x, lastY: y };
    client?.sendCmd({ op: 'mouse', x, y, button: e.button, down: true, click_count: 1 });
  }
  function onMouseMove(e: MouseEvent) {
    if (!dragState.active || !videoEl) return;
    const r = videoEl.getBoundingClientRect();
    const x = ((e.clientX - r.left) / r.width) * 100;
    const y = ((e.clientY - r.top) / r.height) * 100;
    client?.sendCmd({ op: 'mouse', x, y, button: 0, down: true, move: true, dx: x - dragState.lastX, dy: y - dragState.lastY });
    dragState.lastX = x; dragState.lastY = y;
  }
  function onMouseUp(e: MouseEvent) {
    if (!videoEl) return;
    const r = videoEl.getBoundingClientRect();
    const x = ((e.clientX - r.left) / r.width) * 100;
    const y = ((e.clientY - r.top) / r.height) * 100;
    client?.sendCmd({ op: 'mouse', x, y, button: e.button, down: false, click_count: 1 });
    dragState.active = false;
  }
  function onWheel(e: WheelEvent) {
    e.preventDefault();
    client?.sendCmd({ op: 'wheel', dx: e.deltaX, dy: e.deltaY });
  }

  // 触屏（移动）
  function onTouchStart(e: TouchEvent) {
    const t = e.touches[0]; if (!t || !videoEl) return;
    const r = videoEl.getBoundingClientRect();
    const x = ((t.clientX - r.left) / r.width) * 100;
    const y = ((t.clientY - r.top) / r.height) * 100;
    client?.sendCmd({ op: 'mouse', x, y, button: 0, down: true, click_count: 1 });
  }
  function onTouchMove(e: TouchEvent) {
    e.preventDefault();
    const t = e.touches[0]; if (!t || !videoEl) return;
    const r = videoEl.getBoundingClientRect();
    const x = ((t.clientX - r.left) / r.width) * 100;
    const y = ((t.clientY - r.top) / r.height) * 100;
    client?.sendCmd({ op: 'mouse', x, y, move: true });
  }

  // 键盘
  function onKeyDown(e: KeyboardEvent) {
    if (!client) return;
    e.preventDefault();
    client.sendCmd({ op: 'key', code: e.code, key: e.key, down: true, mods: { ctrl: e.ctrlKey, alt: e.altKey, shift: e.shiftKey, meta: e.metaKey } });
  }
  function onKeyUp(e: KeyboardEvent) {
    if (!client) return;
    e.preventDefault();
    client.sendCmd({ op: 'key', code: e.code, key: e.key, down: false, mods: { ctrl: e.ctrlKey, alt: e.altKey, shift: e.shiftKey, meta: e.metaKey } });
  }

  function sendType(text: string) { client?.sendCmd({ op: 'type', text }); }

  function sendLR(left: boolean) { client?.sendCmd({ op: 'mouse', x: 50, y: 50, button: left ? 0 : 2, down: true }); setTimeout(() => client?.sendCmd({ op: 'mouse', x: 50, y: 50, button: left ? 0 : 2, down: false }), 30); }
  function sendWheel(dy: number) { client?.sendCmd({ op: 'wheel', dx: 0, dy }); }

  function togglePrivacy() {
    privacy = !privacy;
    client?.sendCmd({ op: 'privacy', enabled: privacy });
  }
  function toggleAudio() {
    audioMuted = !audioMuted;
    if (videoEl) videoEl.muted = audioMuted;
  }

  async function sendClip() {
    try {
      const text = await navigator.clipboard?.readText();
      if (text) { client?.sendCmd({ op: 'clip', text }); pushLog($t('control.clip.send') + ': ' + text.slice(0, 50)); }
    } catch (e: any) { pushLog('clip send err: ' + e.message); }
  }
  function getClip() {
    client?.sendCmd({ op: 'clip_get' });
    pushLog($t('control.clip.recv') + ' requested');
  }

  // 简单虚拟键盘
  const kbRows = [
    ['Esc','F1','F2','F3','F4','F5','F6','F7','F8','F9','F10','F11','F12'],
    ['`','1','2','3','4','5','6','7','8','9','0','-','=','Backspace'],
    ['Tab','Q','W','E','R','T','Y','U','I','O','P','[',']','\\'],
    ['Caps','A','S','D','F','G','H','J','K','L',';',"'",'Enter'],
    ['Shift','Z','X','C','V','B','N','M',',','.','/','Shift'],
    ['Ctrl','Win','Alt','Space','Alt','Win','Menu','Ctrl']
  ];
  function kp(k: string) {
    if (k === 'Space') sendType(' ');
    else if (k === 'Backspace') client?.sendCmd({ op: 'key', code: 'Backspace', down: true });
    else if (k === 'Enter') client?.sendCmd({ op: 'key', code: 'Enter', down: true });
    else if (k === 'Tab') client?.sendCmd({ op: 'key', code: 'Tab', down: true });
    else if (k === 'Caps' || k === 'Shift' || k === 'Ctrl' || k === 'Alt' || k === 'Win' || k === 'Menu') {
      client?.sendCmd({ op: 'key', code: k, down: true });
    } else sendType(k);
  }

  interface TransferItem { id: string; name: string; total: number; sent: number; paused: boolean; done: boolean; err?: string; }
  let transfers = $state<TransferItem[]>([]);
  let transferAbort = $state<Map<string, AbortController>>(new Map());
  let uploadInput: HTMLInputElement;
  async function onUpload(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f || !client) return;
    const tid = Math.random().toString(36).slice(2, 10);
    const ac = new AbortController();
    transferAbort.set(tid, ac);
    const item: TransferItem = { id: tid, name: f.name, total: f.size, sent: 0, paused: false, done: false };
    transfers = [...transfers, item];
    pushLog('file transfer start: ' + f.name);
    try {
      const { sendFileResumable } = await import('../lib/webrtc');
      await sendFileResumable(client, f, tid, 0, (offset, total) => {
        if (ac.signal.aborted) { throw new Error('canceled'); }
        item.sent = offset; transfers = [...transfers];
      });
      item.done = true; item.sent = item.total; transfers = [...transfers];
      pushLog('file sent: ' + f.name);
    } catch (err: any) {
      if (err.message === 'canceled') { item.paused = true; pushLog('file paused: ' + f.name); }
      else { item.err = err.message; pushLog('file err: ' + err.message); }
      transfers = [...transfers];
    }
  }
  function cancelTransfer(tid: string) {
    const ac = transferAbort.get(tid);
    if (ac) { ac.abort(); transferAbort.delete(tid); }
    transfers = transfers.filter(t => t.id !== tid);
  }
  async function resumeTransfer(tid: string) {
    const t = transfers.find(x => x.id === tid);
    if (!t || !client) return;
    t.paused = false;
    const ac = new AbortController();
    transferAbort.set(tid, ac);
    try {
      const { sendFileResumable } = await import('../lib/webrtc');
      const fileInput = uploadInput;
      const file = fileInput?.files?.[0];
      if (!file) { t.err = 'file input cleared'; transfers = [...transfers]; return; }
      await sendFileResumable(client, file, tid, t.sent, (offset, total) => {
        if (ac.signal.aborted) { throw new Error('canceled'); }
        t.sent = offset; transfers = [...transfers];
      });
      t.done = true; t.sent = t.total; transfers = [...transfers];
      pushLog('file sent: ' + t.name);
    } catch (err: any) {
      if (err.message === 'canceled') { t.paused = true; }
      else { t.err = err.message; }
      transfers = [...transfers];
    }
  }

  function fmtBitrate(kbps: number) { return kbps >= 1000 ? (kbps/1000).toFixed(1) + ' Mbps' : kbps + ' Kbps'; }
</script>

<div class="h-full flex flex-col" tabindex="0" onkeydown={onKeyDown} onkeyup={onKeyUp}>
  <!-- 顶部状态栏 -->
  <header class="flex items-center justify-between px-3 py-2 border-b border-dark-border bg-dark-panel text-sm">
    <div class="flex items-center gap-3">
      <button class="btn-ghost btn-sm" onclick={() => go({ name: 'devices' })}>← {$t('common.back')}</button>
      <span class="font-mono text-xs">{code || '—'}</span>
      <span class="dot {status.startsWith('dc:open') ? 'dot-online' : status.startsWith('signaling:') ? 'dot-busy' : 'dot-offline'}"></span>
      <span class="text-xs text-dark-muted">{status}</span>
    </div>
    <div class="flex items-center gap-3 text-xs text-dark-muted">
      <span>{$t('control.stats.rtt')}: {rtt}ms</span>
      <span>{$t('control.stats.bitrate')}: {fmtBitrate(bitrate)}</span>
      <span>{$t('control.stats.fps')}: {fps}</span>
      <span>{$t('control.stats.codec')}: {codec.toUpperCase()}</span>
      <button class="btn-ghost btn-sm" onclick={toggleAudio}>{audioMuted ? '🔇' : '🔊'} {$t('control.audio')}</button>
      <button class="btn-ghost btn-sm" onclick={() => (showTransfer = !showTransfer)}>{$t('control.transfer')}</button>
      <button class="btn-ghost btn-sm" onclick={() => (showKb = !showKb)}>{$t('control.virtual.kb')}</button>
      <button class="btn-danger btn-sm" onclick={stop}>{$t('common.disconnect')}</button>
    </div>
  </header>

  <div class="flex-1 grid grid-cols-1 md:grid-cols-[1fr_300px] overflow-hidden">
    <!-- 视频/控制画布 -->
    <div class="relative bg-black flex items-center justify-center overflow-hidden">
      {#if !started}
        <div class="w-full max-w-md p-6 space-y-3">
          <h2 class="text-lg font-semibold">{$t('connect.title')}</h2>
          <div>
            <label class="label">{$t('connect.code')}</label>
            <input class="input font-mono uppercase" bind:value={code} placeholder={$t('connect.code.placeholder')} />
          </div>
          <div>
            <label class="label">{$t('connect.password')}</label>
            <input class="input" type="password" bind:value={password} />
          </div>
          <div class="flex items-center gap-3 text-sm">
            <label class="flex items-center gap-1"><input type="radio" bind:group={mode} value="anonymous" /> {$t('connect.mode.anon')}</label>
            <label class="flex items-center gap-1"><input type="radio" bind:group={mode} value="account" /> {$t('connect.mode.account')}</label>
          </div>
          <button class="btn-primary w-full" onclick={start}>{$t('connect.start')}</button>
          <div class="text-[10px] text-dark-muted max-h-32 overflow-auto border border-dark-border rounded p-1 font-mono">
            {#each log as l}<div>{l}</div>{/each}
          </div>
        </div>
      {:else}
        <video bind:this={videoEl}
          class="max-w-full max-h-full"
          style="transform: scale({scale/100}); transform-origin: center;"
          onmousedown={onMouseDown} onmousemove={onMouseMove} onmouseup={onMouseUp}
          onwheel={onWheel}
          ontouchstart={onTouchStart} ontouchmove={onTouchMove}
          autoplay playsinline muted={audioMuted}></video>
        <!-- 触屏浮动控件 -->
        <div class="absolute right-2 bottom-2 flex flex-col gap-2 md:hidden">
          <button class="btn-primary btn-sm" onpointerdown={() => sendLR(true)} onpointerup={() => sendLR(false)}>L</button>
          <button class="btn-primary btn-sm" onpointerdown={() => sendLR(false)} onpointerup={() => sendLR(false)}>R</button>
          <div class="flex flex-col items-center bg-dark-panel/80 rounded p-1">
            <button class="text-xs px-2" onclick={() => sendWheel(-120)}>▲</button>
            <div class="text-[10px] my-1">WH</div>
            <button class="text-xs px-2" onclick={() => sendWheel(120)}>▼</button>
          </div>
        </div>
      {/if}
    </div>

    <!-- 侧边参数 -->
    <aside class="hidden md:flex flex-col border-l border-dark-border bg-dark-panel p-3 gap-3 text-sm overflow-auto">
      <div>
        <label class="label">{$t('control.scale')}: {scale}%</label>
        <input type="range" min="10" max="300" step="5" bind:value={scale} class="w-full" onchange={sendConfig} />
      </div>
      <div>
        <label class="label">{$t('control.bitrate')}: {fmtBitrate(bitrate)}</label>
        <input type="range" min="512" max="200000" step="256" bind:value={bitrate} class="w-full" onchange={sendConfig} />
      </div>
      <div>
        <label class="label">{$t('control.fps')}: {fps}</label>
        <select class="input" bind:value={fps} onchange={sendConfig}>
          {#each [15,30,45,60,75,90,105,120,135,144] as v}<option value={v}>{v}</option>{/each}
        </select>
      </div>
      <div>
        <label class="label">{$t('control.codec')}</label>
        <select class="input" bind:value={codec} onchange={sendConfig}>
          <option value="h264">H.264</option>
          <option value="vp9">VP9</option>
          <option value="av1">AV1</option>
          <option value="vp8">VP8</option>
        </select>
      </div>
      <div class="border-t border-dark-border pt-2">
        <label class="flex items-center gap-2">
          <input type="checkbox" checked={privacy} onchange={togglePrivacy} />
          {$t('connect.privacy')} ({privacy ? $t('control.privacy.on') : $t('control.privacy.off')})
        </label>
      </div>
      <div class="border-t border-dark-border pt-2 space-y-2">
        <button class="btn-ghost w-full" onclick={() => (showKb = !showKb)}>{$t('control.virtual.kb')}</button>
        <button class="btn-ghost w-full" onclick={sendClip}>{$t('control.clip.send')}</button>
        <button class="btn-ghost w-full" onclick={getClip}>{$t('control.clip.recv')}</button>
        <button class="btn-ghost w-full" onclick={() => (showTransfer = !showTransfer)}>{$t('control.transfer')}</button>
        <input type="file" bind:this={uploadInput} onchange={onUpload} class="hidden" />
        <button class="btn-primary w-full" onclick={() => uploadInput?.click()}>{$t('control.send.file')}</button>
      </div>
    </aside>
  </div>

  <!-- 虚拟键盘抽屉 -->
  {#if showKb}
    <div class="border-t border-dark-border bg-dark-panel p-2">
      <div class="flex justify-between items-center mb-2">
        <div class="text-xs text-dark-muted">{$t('control.virtual.kb')}</div>
        <button class="btn-ghost btn-sm" onclick={() => (showKb = false)}>×</button>
      </div>
      <div class="space-y-1">
        {#each kbRows as row}
          <div class="flex gap-1 justify-center">
            {#each row as k}
              <button class="btn-ghost btn-sm min-w-[36px]" onclick={() => kp(k)}>{k}</button>
            {/each}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- 文件传输抽屉 -->
  {#if showTransfer}
    <div class="border-t border-dark-border bg-dark-panel p-3 text-sm">
      <div class="flex justify-between items-center mb-2">
        <div class="font-medium">{$t('control.transfer')}</div>
        <button class="btn-ghost btn-sm" onclick={() => (showTransfer = false)}>×</button>
      </div>
      <div class="space-y-2">
        <button class="btn-primary w-full" onclick={() => uploadInput?.click()}>{$t('control.transfer.upload')}</button>
        {#if transfers.length > 0}
          <div class="max-h-48 overflow-auto space-y-1">
            {#each transfers as t}
              <div class="text-xs bg-dark-border/30 rounded p-2">
                <div class="flex items-center justify-between">
                  <span class="truncate flex-1">{t.name}</span>
                  <span class="text-dark-muted ml-2">{(t.sent / 1024).toFixed(0)}/{t.total > 0 ? (t.total / 1024).toFixed(0) : '?'} KB</span>
                </div>
                <div class="w-full h-1 bg-dark-border rounded mt-1">
                  <div class="h-full bg-primary-500 rounded transition-all" style="width: {t.total > 0 ? (t.sent / t.total * 100) : 0}%"></div>
                </div>
                <div class="flex items-center justify-between mt-1">
                  {#if t.done}
                    <span class="text-green-400">✓ {$t('common.done')}</span>
                  {:else if t.err}
                    <span class="text-rose-400">✗ {t.err}</span>
                    <button class="btn-ghost btn-xs" onclick={() => transfers = transfers.filter(x => x.id !== t.id)}>×</button>
                  {:else if t.paused}
                    <button class="btn-ghost btn-xs" onclick={() => resumeTransfer(t.id)}>{$t('common.resume')}</button>
                    <button class="btn-ghost btn-xs" onclick={() => cancelTransfer(t.id)}>{$t('common.cancel')}</button>
                  {:else}
                    <span class="text-dark-muted">{$t('common.transferring')}</span>
                    <button class="btn-ghost btn-xs" onclick={() => cancelTransfer(t.id)}>{$t('common.pause')}</button>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="text-[11px] text-dark-muted text-center py-4">{$t('control.transfer.queue')}</div>
        {/if}
      </div>
      <input type="file" bind:this={uploadInput} onchange={onUpload} class="hidden" />
    </div>
  {/if}
</div>

<script lang="ts">
  // 浮动工具栏：会话进行中显示
  //  - 拖拽移动（header）
  //  - 切显示器 / 锁屏 / 静音 / 全屏 / 断开
  //  - 状态：在线 / 主控 / 副控 / 录制
  import { invoke } from '@tauri-apps/api/core';
  import { getCurrentWindow, LogicalPosition } from '@tauri-apps/api/window';

  let state = $state<{
    session: 'idle' | 'incoming' | 'active';
    controller: string;
    duration: number;
    muted: boolean;
    fullscreen: boolean;
    recording: boolean;
  }>({ session: 'active', controller: 'kevin-controller', duration: 0, muted: false, fullscreen: false, recording: false });

  let pos = $state({ x: 100, y: 100 });
  let dragging = $state(false);
  let dragOffset = { x: 0, y: 0 };
  let timer: number | undefined;

  onMount(() => {
    // 持续时间秒表
    timer = window.setInterval(() => state.duration++, 1000);
    return () => { if (timer) clearInterval(timer); };
  });

  function onPointerDown(e: PointerEvent) {
    if ((e.target as HTMLElement).closest('button')) return; // 不抢按钮
    dragging = true;
    dragOffset = { x: e.clientX, y: e.clientY };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function onPointerMove(e: PointerEvent) {
    if (!dragging) return;
    pos = { x: pos.x + (e.clientX - dragOffset.x), y: pos.y + (e.clientY - dragOffset.y) };
    dragOffset = { x: e.clientX, y: e.clientY };
  }
  function onPointerUp(e: PointerEvent) {
    dragging = false;
    (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
  }

  async function cycleDisplay() {
    try {
      const ds = await invoke<{ index: number; width: number; height: number; name: string }[]>('list_displays');
      const cur = await invoke<number>('get_selected_display');
      const next = (cur + 1) % ds.length;
      await invoke('select_display', { index: next });
    } catch (e) { console.error(e); }
  }
  async function toggleMute() {
    state.muted = !state.muted;
    // 实际控制走 cmd 发给 controller
  }
  async function toggleFullscreen() {
    state.fullscreen = !state.fullscreen;
  }
  async function toggleRecord() {
    if (state.recording) {
      const path = await invoke<string | null>('stop_recording');
      state.recording = false;
      if (path) console.log('Recording saved to', path);
    } else {
      try {
        const path = await invoke<string>('start_recording');
        state.recording = true;
        console.log('Recording started:', path);
      } catch (e) { console.error('Recording failed:', e); }
    }
  }
  async function endSession() {
    if (!confirm('确认断开当前会话？')) return;
    try { await invoke('stop_service'); } catch (e) { console.error(e); }
  }
  async function hideToolbar() {
    try { await getCurrentWindow().hide(); } catch (e) { console.error(e); }
  }

  function fmtDuration(s: number): string {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const ss = s % 60;
    return (h > 0 ? `${h}:` : '') + String(m).padStart(2, '0') + ':' + String(ss).padStart(2, '0');
  }
</script>

<svelte:window />

<div
  class="fixed top-0 left-0 select-none cursor-move"
  style="transform: translate({pos.x}px, {pos.y}px);"
  onpointerdown={onPointerDown}
  onpointermove={onPointerMove}
  onpointerup={onPointerUp}
>
  <div class="bg-slate-900/95 border border-slate-700 rounded-full shadow-2xl backdrop-blur px-3 py-1.5 flex items-center gap-2 text-xs text-slate-100">
    <span class="w-2 h-2 rounded-full {state.session === 'active' ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'}"></span>
    <span class="font-mono text-[11px]">{state.controller}</span>
    <span class="text-slate-500">·</span>
    <span class="font-mono text-slate-300">{fmtDuration(state.duration)}</span>
    <span class="w-px h-3 bg-slate-700 mx-1"></span>

    <button class="px-2 py-0.5 rounded hover:bg-slate-700 transition" onclick={cycleDisplay} title="切换显示器">🖥</button>
    <button class="px-2 py-0.5 rounded hover:bg-slate-700 transition {state.muted ? 'bg-rose-900/40' : ''}" onclick={toggleMute} title="静音">
      {state.muted ? '🔇' : '🔊'}
    </button>
    <button class="px-2 py-0.5 rounded hover:bg-slate-700 transition" onclick={toggleFullscreen} title="全屏">
      {state.fullscreen ? '⛶' : '⛶'}
    </button>
    <button class="px-2 py-0.5 rounded hover:bg-slate-700 transition {state.recording ? 'bg-rose-900/40' : ''}" onclick={toggleRecord} title="录屏">
      {state.recording ? '⏺' : '⏺'}
    </button>
    <span class="w-px h-3 bg-slate-700 mx-1"></span>
    <button class="px-2 py-0.5 rounded hover:bg-rose-700 text-rose-200 hover:text-white transition" onclick={endSession} title="断开会话">⏻</button>
    <button class="px-2 py-0.5 rounded hover:bg-slate-700 text-slate-400 hover:text-slate-200 transition" onclick={hideToolbar} title="隐藏工具栏">×</button>
  </div>
</div>

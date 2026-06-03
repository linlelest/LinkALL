<script lang="ts">
  type Page = 'dashboard' | 'crash' | 'devices' | 'fcm' | 'rate' | 'audit';
  let { active, ...rest }: { active: Page } = $props();
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher<{ nav: Page; logout: void }>();

  const items: { id: Page; label: string; icon: string }[] = [
    { id: 'dashboard', label: '概览',     icon: '📊' },
    { id: 'crash',     label: '崩溃日志', icon: '🛑' },
    { id: 'devices',   label: '设备',     icon: '🖥️' },
    { id: 'fcm',       label: 'FCM 令牌', icon: '🔔' },
    { id: 'rate',      label: '限流',     icon: '🚦' },
    { id: 'audit',     label: '审计',     icon: '📜' },
  ];
</script>

<aside class="w-56 bg-slate-900 border-r border-slate-800 flex flex-col">
  <div class="px-4 py-4 border-b border-slate-800">
    <h1 class="text-lg font-bold tracking-wide">LinkALL</h1>
    <p class="text-xs text-slate-500">Admin Console</p>
  </div>
  <nav class="flex-1 p-2 space-y-1">
    {#each items as it}
      <button
        class="w-full text-left px-3 py-2 rounded text-sm flex items-center gap-2 transition
               {active === it.id ? 'bg-sky-600/30 text-sky-200 border-l-2 border-sky-400' : 'hover:bg-slate-800 text-slate-300'}"
        onclick={() => dispatch('nav', it.id)}
      >
        <span>{it.icon}</span>
        <span>{it.label}</span>
      </button>
    {/each}
  </nav>
  <div class="p-3 border-t border-slate-800">
    <button class="btn btn-ghost w-full justify-center" onclick={() => dispatch('logout')}>
      退出
    </button>
  </div>
</aside>

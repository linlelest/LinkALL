<script lang="ts">
  import { getToken, setToken } from './lib/api.svelte';
  import Login from './lib/Login.svelte';
  import Sidebar from './lib/Sidebar.svelte';
  import Dashboard from './lib/Dashboard.svelte';
  import CrashLogs from './lib/CrashLogs.svelte';
  import Devices from './lib/Devices.svelte';
  import FcmTokens from './lib/FcmTokens.svelte';
  import RateLimit from './lib/RateLimit.svelte';
  import Audit from './lib/Audit.svelte';

  type Page = 'dashboard' | 'crash' | 'devices' | 'fcm' | 'rate' | 'audit';
  let active = $state<Page>('dashboard');

  function nav(p: Page) { active = p; }
  function logout() { setToken(null); }
</script>

{#if !getToken()}
  <div class="min-h-screen flex items-center justify-center">
    <Login />
  </div>
{:else}
  <div class="flex h-screen">
    <Sidebar {active} on:nav={(e) => nav(e.detail)} on:logout={logout} />
    <main class="flex-1 overflow-y-auto p-6 bg-slate-950">
      {#if active === 'dashboard'}<Dashboard />{/if}
      {#if active === 'crash'}<CrashLogs />{/if}
      {#if active === 'devices'}<Devices />{/if}
      {#if active === 'fcm'}<FcmTokens />{/if}
      {#if active === 'rate'}<RateLimit />{/if}
      {#if active === 'audit'}<Audit />{/if}
    </main>
  </div>
{/if}

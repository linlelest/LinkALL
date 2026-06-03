<script lang="ts">
  import { onMount } from 'svelte';
  import { route, go, type Route } from './lib/router';
  import { token, user, logout } from './lib/auth';
  import { locale, setLocale, t } from './i18n';
  import Login from './routes/Login.svelte';
  import Dashboard from './routes/Dashboard.svelte';
  import Devices from './routes/Devices.svelte';
  import Connect from './routes/Connect.svelte';
  import Control from './routes/Control.svelte';
  import Announcements from './routes/Announcements.svelte';
  import Profile from './routes/Profile.svelte';
  import Help from './routes/Help.svelte';
  import AdminUsers from './routes/admin/Users.svelte';
  import AdminInvites from './routes/admin/Invites.svelte';
  import AdminAnnouncements from './routes/admin/Announcements.svelte';
  import AdminOTA from './routes/admin/OTA.svelte';
  import AdminServer from './routes/admin/Server.svelte';

  let mobileNavOpen = $state(false);
  let adminNavOpen = $state(true);

  const navItems: Array<{ key: any; route: Route; icon: string }> = [
    { key: 'nav.dashboard', route: { name: 'dashboard' }, icon: '⌂' },
    { key: 'nav.devices', route: { name: 'devices' }, icon: '▣' },
    { key: 'nav.connect', route: { name: 'connect' }, icon: '⇄' },
    { key: 'nav.announcements', route: { name: 'announcements' }, icon: '✉' },
    { key: 'nav.profile', route: { name: 'profile' }, icon: '◉' },
    { key: 'nav.help', route: { name: 'help' }, icon: '?' }
  ];

  onMount(() => {
    if (!$token) go({ name: 'login' });
  });

  function navClick(r: Route) {
    mobileNavOpen = false;
    go(r);
  }

  function isActive(r: Route): boolean {
    if ($route.name === r.name) {
      if (r.name === 'admin' && (r as any).section !== ($route as any).section) return false;
      return true;
    }
    return false;
  }
</script>

{#if $route.name === 'login' || !$token}
  <Login />
{:else if $route.name === 'control'}
  <Control deviceCode={($route as any).deviceCode} />
{:else}
  <div class="h-full flex flex-col md:flex-row">
    <!-- 侧边栏（桌面） -->
    <aside class="hidden md:flex flex-col w-56 border-r border-dark-border bg-dark-panel">
      <div class="px-4 py-4 border-b border-dark-border">
        <div class="text-lg font-semibold text-primary-400">LinkALL</div>
        <div class="text-xs text-dark-muted">{$t('app.tagline')}</div>
      </div>
      <nav class="flex-1 p-2 space-y-1">
        {#each navItems as it}
          <button class="w-full text-left px-3 py-2 rounded text-sm flex items-center gap-2 {isActive(it.route) ? 'bg-primary-500/20 text-primary-300' : 'hover:bg-dark-border/50'}"
            onclick={() => navClick(it.route)}>
            <span class="text-primary-400 w-4 text-center">{it.icon}</span>
            <span>{$t(it.key)}</span>
          </button>
        {/each}
        {#if $user?.is_admin}
          {@const ar = { name: 'admin', section: 'users' } as Route}
          <div class="pt-2 mt-2 border-t border-dark-border">
            <button class="w-full text-left px-3 py-2 rounded text-sm flex items-center gap-2 {$route.name === 'admin' ? 'bg-primary-500/20 text-primary-300' : 'hover:bg-dark-border/50'}"
              onclick={() => navClick(ar)}>
              <span class="text-primary-400 w-4 text-center">⚙</span>
              <span>{$t('nav.admin')}</span>
            </button>
          </div>
        {/if}
      </nav>
      <div class="p-2 border-t border-dark-border text-xs">
        <select class="input" value={$locale} onchange={(e) => setLocale((e.target as HTMLSelectElement).value as any)}>
          <option value="zh-CN">中文</option>
          <option value="en-US">English</option>
        </select>
        <button class="btn-ghost w-full mt-2 text-xs" onclick={() => { logout(); go({ name: 'login' }); }}>{$t('nav.logout')}</button>
        <div class="text-dark-muted text-[10px] mt-1 text-center">{$user?.username}</div>
      </div>
    </aside>

    <!-- 顶部栏（移动） -->
    <header class="md:hidden flex items-center justify-between p-3 border-b border-dark-border bg-dark-panel">
      <button class="text-xl" onclick={() => (mobileNavOpen = !mobileNavOpen)} aria-label="menu">☰</button>
      <div class="font-semibold text-primary-400">LinkALL</div>
      <button class="text-sm" onclick={() => { logout(); go({ name: 'login' }); }}>⎋</button>
    </header>

    <!-- 移动菜单 -->
    {#if mobileNavOpen}
      <div class="md:hidden absolute inset-0 z-40 bg-black/60" onclick={() => (mobileNavOpen = false)} role="presentation">
        <div class="w-56 h-full bg-dark-panel p-2 space-y-1" onclick={(e) => e.stopPropagation()} role="presentation">
          {#each navItems as it}
            <button class="w-full text-left px-3 py-2 rounded text-sm {isActive(it.route) ? 'bg-primary-500/20 text-primary-300' : 'hover:bg-dark-border/50'}"
              onclick={() => navClick(it.route)}>
              {$t(it.key)}
            </button>
          {/each}
          {#if $user?.is_admin}
            <button class="w-full text-left px-3 py-2 rounded text-sm {$route.name === 'admin' ? 'bg-primary-500/20 text-primary-300' : 'hover:bg-dark-border/50'}"
              onclick={() => navClick({ name: 'admin', section: 'users' })}>
              {$t('nav.admin')}
            </button>
          {/if}
          <select class="input mt-2" value={$locale} onchange={(e) => setLocale((e.target as HTMLSelectElement).value as any)}>
            <option value="zh-CN">中文</option>
            <option value="en-US">English</option>
          </select>
        </div>
      </div>
    {/if}

    <main class="flex-1 overflow-auto">
      {#if $route.name === 'dashboard'}
        <Dashboard />
      {:else if $route.name === 'devices'}
        <Devices />
      {:else if $route.name === 'connect'}
        <Connect />
      {:else if $route.name === 'announcements'}
        <Announcements />
      {:else if $route.name === 'profile'}
        <Profile />
      {:else if $route.name === 'help'}
        <Help />
      {:else if $route.name === 'admin'}
        <div class="h-full grid grid-cols-1" class:md:grid-cols-[180px_1fr]={adminNavOpen} class:md:grid-cols-[40px_1fr]={!adminNavOpen}>
          <nav class="md:border-r border-dark-border bg-dark-panel p-1 space-y-1">
            <button class="w-full text-left px-2 py-1 rounded text-xs text-dark-muted hover:bg-dark-border/50" onclick={() => adminNavOpen = !adminNavOpen}>
              {adminNavOpen ? '◀' : '▶'}
            </button>
            {#if adminNavOpen}
              {#each [
                {k:'nav.admin.users', s:'users'},
                {k:'nav.admin.invites', s:'invites'},
                {k:'nav.admin.announcements', s:'announcements'},
                {k:'nav.admin.ota', s:'ota'},
                {k:'nav.admin.server', s:'server'}
              ] as it}
                <button class="w-full text-left px-3 py-2 rounded text-sm {($route as any).section === it.s ? 'bg-primary-500/20 text-primary-300' : 'hover:bg-dark-border/50'}"
                  onclick={() => go({ name: 'admin', section: it.s as any })}>
                  {$t(it.k)}
                </button>
              {/each}
            {/if}
          </nav>
          <div class="p-4 md:p-6 overflow-auto">
            {#if ($route as any).section === 'users'}
              <AdminUsers />
            {:else if ($route as any).section === 'invites'}
              <AdminInvites />
            {:else if ($route as any).section === 'announcements'}
              <AdminAnnouncements />
            {:else if ($route as any).section === 'ota'}
              <AdminOTA />
            {:else if ($route as any).section === 'server'}
              <AdminServer />
            {/if}
          </div>
        </div>
      {/if}
    </main>
  </div>
{/if}

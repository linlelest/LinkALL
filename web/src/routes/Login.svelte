<script lang="ts">
  import { t, setLocale, getLocale } from '../i18n';
  import { api, setServer } from '../lib/api';
  import { token, user } from '../lib/auth';
  import { go } from '../lib/router';
  import Alert from '../lib/components/Alert.svelte';

  let action = $state<'login' | 'register'>('login');
  let username = $state('');
  let password = $state('');
  let invite = $state('');
  let server = $state(localStorage.getItem('linkall.server') || '');
  let err = $state('');
  let loading = $state(false);

  async function submit() {
    err = '';
    if (server.trim()) setServer(server.trim());
    loading = true;
    try {
      const data = await api.post<{token:string; user:any}>('/api/auth/login', {
        action, username, password, invite_code: invite
      });
      token.set(data.token);
      user.set(data.user);
      const loc = data.user?.locale;
      if (loc === 'zh-CN' || loc === 'en-US') setLocale(loc);
      go({ name: 'dashboard' });
    } catch (e: any) {
      err = e.message || $t('login.failed');
    } finally {
      loading = false;
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center p-4">
  <div class="w-full max-w-sm card p-6">
    <div class="text-center mb-4">
      <div class="text-2xl font-bold text-primary-400">LinkALL</div>
      <div class="text-xs text-dark-muted">{$t('app.tagline')}</div>
    </div>
    <div class="flex border-b border-dark-border mb-4">
      <button class="tab {action === 'login' ? 'tab-active' : ''}" onclick={() => (action = 'login')}>{$t('login.tab.login')}</button>
      <button class="tab {action === 'register' ? 'tab-active' : ''}" onclick={() => (action = 'register')}>{$t('login.tab.register')}</button>
    </div>
    <form onsubmit={(e) => { e.preventDefault(); submit(); }} class="space-y-3">
      <div>
        <label class="label">{$t('login.username')}</label>
        <input class="input" bind:value={username} required minlength="3" maxlength="32" autocomplete="username" />
      </div>
      <div>
        <label class="label">{$t('login.password')}</label>
        <input class="input" type="password" bind:value={password} required minlength="6" autocomplete="current-password" />
      </div>
      {#if action === 'register'}
        <div>
          <label class="label">{$t('login.invite')}</label>
          <input class="input" bind:value={invite} required placeholder="12345678" />
          <div class="text-[10px] text-dark-muted mt-1">{$t('login.invite.hint')}</div>
        </div>
      {/if}
      <div>
        <label class="label">{$t('login.server')}</label>
        <input class="input" bind:value={server} placeholder="http://127.0.0.1:8080" />
        <div class="text-[10px] text-dark-muted mt-1">{$t('login.server.hint')}</div>
      </div>
      {#if err}
        <Alert variant="error" message={err} />
      {/if}
      <button class="btn-primary w-full" type="submit" disabled={loading}>
        {loading ? $t('common.loading') : (action === 'login' ? $t('login.submit') : $t('login.register'))}
      </button>
    </form>
  </div>
</div>

<script lang="ts">
  import { t, setLocale } from '../i18n';
  import { api } from '../lib/api';
  import { user, token } from '../lib/auth';
  import Alert from '../lib/components/Alert.svelte';

  let oldPw = $state('');
  let newPw = $state('');
  let msg = $state<{kind:'ok'|'err', text:string}|null>(null);

  async function changePw() {
    msg = null;
    try {
      await api.post('/api/auth/password', { old_password: oldPw, new_password: newPw });
      msg = { kind: 'ok', text: $t('profile.saved') };
      oldPw = ''; newPw = '';
    } catch (e: any) {
      msg = { kind: 'err', text: e.message };
    }
  }
  async function setLoc(v: string) {
    setLocale(v as any);
    try { await api.post('/api/auth/locale', { locale: v }); } catch {}
  }
</script>

<div class="p-4 md:p-6 max-w-2xl space-y-4">
  <h1 class="text-xl font-semibold">{$t('profile.title')}</h1>
  <div class="card p-4 space-y-3">
    <div>
      <label class="label">{$t('profile.username')}</label>
      <input class="input" value={$user?.username || ''} disabled />
    </div>
    <div>
      <label class="label">{$t('profile.locale')}</label>
      <select class="input" value={$user?.locale || 'zh-CN'} onchange={(e) => setLoc((e.target as HTMLSelectElement).value)}>
        <option value="zh-CN">中文</option>
        <option value="en-US">English</option>
      </select>
    </div>
  </div>
  <div class="card p-4 space-y-3">
    <h2 class="font-medium">{$t('profile.password.change')}</h2>
    <div>
      <label class="label">{$t('profile.password.old')}</label>
      <input class="input" type="password" bind:value={oldPw} />
    </div>
    <div>
      <label class="label">{$t('profile.password.new')}</label>
      <input class="input" type="password" bind:value={newPw} />
    </div>
    <button class="btn-primary" onclick={changePw}>{$t('common.save')}</button>
    {#if msg}
      <Alert variant={msg.kind === 'ok' ? 'success' : 'error'} message={msg.text} />
    {/if}
  </div>
</div>

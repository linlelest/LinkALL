<script lang="ts">
  import { api, ApiError, setToken } from '../lib/api.svelte';

  let username = $state('admin');
  let password = $state('');
  let err = $state<string | null>(null);
  let busy = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    err = null;
    busy = true;
    try {
      const r = await api.login(username, password);
      setToken(r.token);
      // token 更新后 App.svelte 会自动重渲染
    } catch (e: any) {
      if (e instanceof ApiError) err = e.body?.error || `${e.status} ${e.message}`;
      else err = e?.message || String(e);
    } finally {
      busy = false;
    }
  }
</script>

<form on:submit={submit} class="card w-96 max-w-full flex flex-col gap-3">
  <h1 class="text-xl font-semibold mb-1">LinkALL Admin 登录</h1>
  <div>
    <span class="label">用户名</span>
    <input class="input" bind:value={username} autocomplete="username" required />
  </div>
  <div>
    <span class="label">密码</span>
    <input class="input" type="password" bind:value={password} autocomplete="current-password" required />
  </div>
  {#if err}
    <p class="text-rose-400 text-sm">{err}</p>
  {/if}
  <button class="btn btn-primary justify-center mt-2" disabled={busy}>
    {busy ? '登录中…' : '登录'}
  </button>
  <p class="text-xs text-slate-500 mt-1">提示：admin 用户由服务端 <code>linkall-server init-admin</code> 创建</p>
</form>

<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../../i18n';
  import { api } from '../../lib/api';
  import Alert from '../../lib/components/Alert.svelte';

  interface User { id: number; username: string; is_admin: boolean; is_super_admin: boolean; banned: boolean; last_login_ip: string; last_login_at: number; }
  let users = $state<User[]>([]);
  let err = $state('');

  onMount(async () => {
    try { users = await api.get<User[]>('/api/admin/users'); } catch (e: any) { err = e.message; }
  });

  async function toggleBan(u: User) {
    try { await api.patch(`/api/admin/users/${u.id}`, { banned: !u.banned }); u.banned = !u.banned; users = [...users]; } catch (e: any) { err = e.message; }
  }
  async function toggleAdmin(u: User) {
    try { await api.patch(`/api/admin/users/${u.id}`, { is_admin: !u.is_admin, is_super_admin: u.is_super_admin }); u.is_admin = !u.is_admin; users = [...users]; } catch (e: any) { err = e.message; }
  }
  async function resetPwd(u: User) {
    const np = prompt('新密码 (>=6 chars)');
    if (!np) return;
    try { await api.patch(`/api/admin/users/${u.id}`, { new_password: np }); alert('OK'); } catch (e: any) { err = e.message; }
  }
  async function delUser(u: User) {
    if (!confirm('Delete ' + u.username + '?')) return;
    try { await api.del(`/api/admin/users/${u.id}`); users = users.filter(x => x.id !== u.id); } catch (e: any) { err = e.message; }
  }
  function fmt(ts: number) { return ts ? new Date(ts * 1000).toLocaleString() : '-'; }
</script>

<h1 class="text-xl font-semibold mb-4">{$t('admin.users.title')}</h1>
{#if err}<Alert variant="error" message={err} />{/if}
<div class="card overflow-hidden">
  <table class="w-full text-sm">
    <thead class="bg-dark-border/40 text-xs text-dark-muted">
      <tr>
        <th class="text-left px-3 py-2">#</th>
        <th class="text-left px-3 py-2">{$t('login.username')}</th>
        <th class="text-left px-3 py-2">{$t('admin.users.admin')}</th>
        <th class="text-left px-3 py-2">{$t('admin.users.super')}</th>
        <th class="text-left px-3 py-2">{$t('admin.users.banned')}</th>
        <th class="text-left px-3 py-2">{$t('admin.users.last_login')}</th>
        <th class="text-right px-3 py-2">{$t('common.more')}</th>
      </tr>
    </thead>
    <tbody>
      {#each users as u}
        <tr class="border-t border-dark-border/60">
          <td class="px-3 py-2 text-xs">{u.id}</td>
          <td class="px-3 py-2 font-mono text-xs">{u.username}</td>
          <td class="px-3 py-2"><input type="checkbox" checked={u.is_admin} onchange={() => toggleAdmin(u)} /></td>
          <td class="px-3 py-2">{u.is_super_admin ? '★' : ''}</td>
          <td class="px-3 py-2"><input type="checkbox" checked={u.banned} onchange={() => toggleBan(u)} /></td>
          <td class="px-3 py-2 text-[11px] text-dark-muted">{fmt(u.last_login_at)}<br/><span class="text-[10px]">{u.last_login_ip}</span></td>
          <td class="px-3 py-2 text-right">
            <button class="btn-ghost btn-sm" onclick={() => resetPwd(u)}>{$t('admin.users.reset_pwd')}</button>
            <button class="btn-danger btn-sm" onclick={() => delUser(u)}>{$t('common.delete')}</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
  {#if users.length > 0 && !users.some(u => u.is_super_admin)}
    <div class="px-3 py-2 text-[11px] text-amber-400 bg-amber-500/5 border-t border-dark-border/60">
      ⚠ {$t('admin.users.no_super')}
    </div>
  {/if}
</div>

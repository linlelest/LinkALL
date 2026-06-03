<script lang="ts">
  import { t } from '../i18n';
</script>

<div class="p-4 md:p-6 max-w-2xl mx-auto">
  <h1 class="text-xl font-semibold mb-4">{$t('help.title')}</h1>

  <h2 class="font-medium mt-6 mb-2">{$t('help.shortcuts')}</h2>
  <div class="card p-4 text-sm space-y-2">
    {#each [
      {k:'help.shcut.esc'},
      {k:'help.shcut.f11'},
      {k:'help.shcut.ctrl_c'},
      {k:'help.shcut.ctrl_v'},
      {k:'help.shcut.ctrl_alt_del'},
      {k:'help.shcut.clip_send'},
      {k:'help.shcut.clip_recv'},
    ] as s}
      <div class="flex items-center gap-3">
        <span class="text-dark-muted w-8">•</span>
        <span>{$t(s.k)}</span>
      </div>
    {/each}
  </div>
</div>

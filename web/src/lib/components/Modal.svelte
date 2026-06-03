<script lang="ts">
  import { t } from '../i18n';
  interface Props { open: boolean; title?: string; onclose?: () => void; }
  let { open = $bindable(), title = '', onclose }: Props = $props();
  function close() { open = false; onclose?.(); }
</script>

{#if open}
  <div class="modal-bg" onclick={close} role="presentation">
    <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog">
      {#if title}
        <div class="px-4 py-3 border-b border-dark-border text-sm font-medium">{$t(title)}</div>
      {/if}
      <div class="p-4">
        <slot />
      </div>
      <div class="px-4 py-3 border-t border-dark-border flex justify-end gap-2">
        <slot name="actions" />
      </div>
    </div>
  </div>
{/if}

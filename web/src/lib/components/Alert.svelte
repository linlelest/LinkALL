<script lang="ts">
  interface Props { variant?: 'success' | 'error' | 'info'; message: string; }
  let { variant = 'info', message }: Props = $props();
  const cls = $derived(variant === 'error' ? 'bg-rose-500/15 text-rose-300 border-rose-500/30'
    : variant === 'success' ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'
    : 'bg-primary-500/10 text-primary-300 border-primary-500/30');
</script>

<div class="border rounded px-3 py-2 text-sm {cls}" role="alert">{message}</div>

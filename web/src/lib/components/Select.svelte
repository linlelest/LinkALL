<script lang="ts">
  import { t } from '../i18n';
  let { value = $bindable(), options }: { value: any; options: Array<{label:string, value:any}> } = $props();
</script>

<select bind:value class="input">
  {#each options as o}
    <option value={o.value}>{$t(o.label, o.label)}</option>
  {/each}
</select>

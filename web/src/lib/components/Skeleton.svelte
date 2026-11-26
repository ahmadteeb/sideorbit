<script lang="ts">
  let { cards = 2 }: { cards?: number } = $props();
</script>

<div class="stack" role="status">
  <span class="sr-only">Loading…</span>
  {#each { length: cards } as _, i (i)}
    <div class="card stack" aria-hidden="true">
      <div class="bar short"></div>
      <div class="bar"></div>
      <div class="bar"></div>
    </div>
  {/each}
</div>

<style>
  .bar {
    height: var(--space-4);
    border-radius: var(--radius-sm);
    background: var(--color-surface-2);
  }
  .short {
    width: 40%;
  }
</style>

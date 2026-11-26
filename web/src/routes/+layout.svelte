<script lang="ts">
  import '$lib/styles/tokens.css';

  let { children } = $props();
</script>

{@render children()}

<style>
  :global(body) {
    margin: 0;
    background: var(--color-bg);
    color: var(--color-text);
    font-family: var(--font-sans);
    line-height: 1.5;
  }
</style>

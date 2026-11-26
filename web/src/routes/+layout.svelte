<script lang="ts">
  import '@fontsource/ibm-plex-sans/400.css';
  import '@fontsource/ibm-plex-sans/500.css';
  import '@fontsource/ibm-plex-sans/600.css';
  import '@fontsource/jetbrains-mono/400.css';
  import '$lib/styles/tokens.css';
  import '$lib/styles/app.css';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';

  let { children } = $props();
</script>

{#if page.url.pathname === '/login'}
  {@render children()}
{:else}
  <Shell>{@render children()}</Shell>
{/if}

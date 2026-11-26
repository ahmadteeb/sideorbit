<script lang="ts">
  import '@fontsource/ibm-plex-sans/400.css';
  import '@fontsource/ibm-plex-sans/500.css';
  import '@fontsource/ibm-plex-sans/600.css';
  import '@fontsource/jetbrains-mono/400.css';
  import '$lib/styles/tokens.css';
  import '$lib/styles/app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { applyTheme, savedTheme } from '$lib/theme';
  import Shell from '$lib/components/Shell.svelte';

  let { children } = $props();

  onMount(() => applyTheme(savedTheme()));
</script>

{#if page.url.pathname === '/login'}
  {@render children()}
{:else}
  <Shell>{@render children()}</Shell>
{/if}

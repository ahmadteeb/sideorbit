<!-- Empty and error states: one icon, a sentence and a next step. -->
<script lang="ts">
  import type { Component, Snippet } from 'svelte';

  let {
    icon: Icon,
    title,
    text = '',
    alert = false,
    children
  }: {
    icon: Component<{ size?: number; strokeWidth?: number; 'aria-hidden'?: 'true' }>;
    title: string;
    text?: string;
    alert?: boolean;
    children?: Snippet;
  } = $props();
</script>

<div class="notice card" role={alert ? 'alert' : undefined}>
  <Icon size={28} strokeWidth={1.75} aria-hidden="true" />
  <h2>{title}</h2>
  {#if text}<p class="muted">{text}</p>{/if}
  {@render children?.()}
</div>

<style>
  .notice {
    display: grid;
    justify-items: center;
    gap: var(--space-3);
    padding-block: var(--space-8);
    text-align: center;
  }
</style>

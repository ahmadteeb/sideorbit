<!-- Left rail from 1024px, bottom tab bar below it. -->
<script lang="ts">
  import { History, LayoutDashboard, Library, Settings } from '@lucide/svelte';
  import type { Snippet } from 'svelte';
  import { page } from '$app/state';

  let { children }: { children: Snippet } = $props();

  const items = [
    { href: '/', label: 'Home', icon: LayoutDashboard },
    { href: '/activity', label: 'Activity', icon: History },
    { href: '/sources', label: 'Sources', icon: Library },
    { href: '/settings', label: 'Settings', icon: Settings }
  ];
  const active = (href: string) =>
    href === '/'
      ? page.url.pathname === '/' || page.url.pathname.startsWith('/devices')
      : page.url.pathname.startsWith(href);
</script>

<a class="skip btn" href="#main">Skip to content</a>
<nav aria-label="Main">
  <span class="brand" aria-hidden="true">Orbit</span>
  <ul class="list">
    {#each items as item (item.href)}
      <li>
        <a href={item.href} aria-current={active(item.href) ? 'page' : undefined}>
          <item.icon size={20} strokeWidth={1.75} aria-hidden="true" />
          <span class="caption">{item.label}</span>
        </a>
      </li>
    {/each}
  </ul>
</nav>
<main id="main" tabindex="-1">
  {@render children()}
</main>

<style>
  main {
    --gutter: var(--space-4);
    max-width: 60rem;
    margin-inline: auto;
    padding: max(var(--space-6), env(safe-area-inset-top))
      max(var(--gutter), env(safe-area-inset-right))
      calc(5.5rem + env(safe-area-inset-bottom)) max(var(--gutter), env(safe-area-inset-left));
  }
  main:focus {
    outline: none;
  }
  nav {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 1;
    background: var(--color-surface);
    border-top: 1px solid var(--color-border);
    padding: var(--space-1) env(safe-area-inset-right) env(safe-area-inset-bottom)
      env(safe-area-inset-left);
  }
  .brand {
    display: none;
  }
  ul {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-2);
  }
  nav :global(.list > li) {
    padding: 0;
    border: 0;
  }
  a[href]:not(.skip) {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-1);
    min-height: 3.25rem;
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    text-decoration: none;
  }
  a[href]:active {
    opacity: 0.7;
  }
  a[aria-current='page'] {
    background: var(--color-surface-2);
    color: var(--color-text);
  }
  a[aria-current='page'] span {
    text-decoration: underline;
    text-underline-offset: var(--space-1);
  }
  .skip {
    position: absolute;
    top: var(--space-2);
    left: var(--space-2);
    z-index: 2;
    transform: translateY(-200%);
  }
  .skip:focus {
    transform: none;
  }
  @media (min-width: 430px) {
    main {
      --gutter: var(--space-6);
    }
  }
  @media (min-width: 768px) {
    main {
      --gutter: var(--space-8);
    }
  }
  @media (min-width: 1024px) {
    nav {
      inset: 0 auto 0 0;
      width: 14rem;
      border-top: 0;
      border-right: 1px solid var(--color-border);
      padding: var(--space-4);
    }
    .brand {
      display: block;
      padding: var(--space-2) var(--space-3) var(--space-4);
      font-size: 1.375rem;
      font-weight: 600;
    }
    ul {
      grid-template-columns: 1fr;
    }
    a[href]:not(.skip) {
      flex-direction: row;
      justify-content: flex-start;
      gap: var(--space-3);
      min-height: var(--target-min);
      padding-inline: var(--space-3);
    }
    main {
      margin-left: max(14rem, calc((100vw - 60rem) / 2));
      padding-bottom: var(--space-12);
    }
  }
</style>

<svelte:head><title>Orbit</title></svelte:head>

<main>
  <h1>Orbit</h1>
  <p>The dashboard isn't built yet. It's coming in a later release.</p>
</main>

<style>
  main {
    --gutter: var(--space-4);
    padding: max(var(--space-8), env(safe-area-inset-top)) max(var(--gutter), env(safe-area-inset-right))
      max(var(--space-8), env(safe-area-inset-bottom)) max(var(--gutter), env(safe-area-inset-left));
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
  h1 {
    margin: 0 0 var(--space-4);
    font-size: 2rem;
    line-height: 2.5rem;
    font-weight: 600;
  }
  p {
    color: var(--color-text-muted);
  }
</style>

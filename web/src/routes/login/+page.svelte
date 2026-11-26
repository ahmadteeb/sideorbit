<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, asApiError } from '$lib/api';
  import Loaded from '$lib/components/Loaded.svelte';
  import { resource } from '$lib/resource.svelte';
  import type { Methods } from '$lib/types';

  const methods = resource('methods', () => api<Methods>('GET', '/api/auth/methods'));

  const errors: Record<string, string> = {
    access_denied: 'Sign-in was cancelled. Try again when you are ready.',
    email_unverified: 'That account has no verified email. Verify it there, or use an email link.',
    account_exists: 'You already have an Orbit account with this email. Sign in the way you did before, or use an email link.',
    provider_error: "The sign-in service didn't respond. Try again."
  };
  const code = page.url.searchParams.get('error');
  const returned = code ? (errors[code] ?? errors.provider_error) : '';

  // Already signed in? Skip the page. A 401 is the normal case here.
  fetch('/api/me', { credentials: 'same-origin' }).then(
    (res) => {
      if (res.ok) void goto('/');
    },
    () => {}
  );

  let email = $state('');
  let error = $state('');
  let busy = $state(false);
  let sent = $state(false);

  function validate(): boolean {
    error = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) ? '' : 'Enter an email address, like name@example.com';
    return !error;
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!validate()) return;
    busy = true;
    try {
      await api('POST', '/api/auth/email/start', { email: email.trim() });
      sent = true;
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Sign in · Orbit</title></svelte:head>

<main class="sections">
  <header class="stack">
    <h1>Sign in to Orbit</h1>
    <p class="muted">See every device, app and expiry date. This is your Orbit account, not your Apple ID.</p>
  </header>

  {#if returned}<p class="card danger-text" role="alert">{returned}</p>{/if}

  <Loaded of={methods}>
    {#snippet children(available)}
      <div class="stack">
        <!-- Full page loads: these leave the app. -->
        {#if available.github}<a class="btn" href="/api/auth/github/start" data-sveltekit-reload>Continue with GitHub</a>{/if}
        {#if available.google}<a class="btn" href="/api/auth/google/start" data-sveltekit-reload>Continue with Google</a>{/if}

        {#if available.email}
          {#if sent}
            <div class="card stack" role="status">
              <h2>Check your inbox</h2>
              <p>We sent a sign-in link to {email}. It works once and expires in 15 minutes.</p>
              <div><button class="btn" type="button" onclick={() => (sent = false)}>Use a different address</button></div>
            </div>
          {:else}
            <form class="stack" onsubmit={submit} novalidate>
              <div>
                <label for="email">Email</label>
                <input
                  id="email"
                  type="email"
                  autocomplete="email"
                  bind:value={email}
                  onblur={() => email && validate()}
                  aria-describedby="email-error"
                  aria-invalid={error ? 'true' : undefined}
                />
                <p class="danger-text field-error" id="email-error" role="alert">{error}</p>
              </div>
              <button class="btn btn-primary" type="submit" disabled={busy}>
                {busy ? 'Sending…' : 'Email me a sign-in link'}
              </button>
            </form>
          {/if}
        {/if}
      </div>
    {/snippet}
  </Loaded>

  <section class="card stack" aria-labelledby="stores">
    <h2 id="stores">What this server stores</h2>
    <p>Your email address, device names, app names, versions and expiry dates, refresh history and a masked Apple ID label.</p>
    <p><strong>Never</strong> your Apple ID password, verification codes, certificates or pairing files. They stay on your iPhone.</p>
  </section>
</main>

<style>
  main {
    max-width: 28rem;
    margin-inline: auto;
    padding: max(var(--space-8), env(safe-area-inset-top))
      max(var(--space-4), env(safe-area-inset-right))
      max(var(--space-8), env(safe-area-inset-bottom))
      max(var(--space-4), env(safe-area-inset-left));
  }
</style>

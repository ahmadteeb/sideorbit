<script lang="ts">
  import { LogOut, Trash2 } from '@lucide/svelte';
  import { goto } from '$app/navigation';
  import { api, asApiError } from '$lib/api';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import PushSettings from '$lib/components/PushSettings.svelte';
  import { clearCache, resource } from '$lib/resource.svelte';
  import { dateTime } from '$lib/status';
  import { applyTheme, savedTheme, type Theme } from '$lib/theme';
  import type { Me, Session } from '$lib/types';

  const account = resource('settings', async () => {
    const [me, { sessions }] = await Promise.all([
      api<Me>('GET', '/api/me'),
      api<{ sessions: Session[] }>('GET', '/api/me/sessions')
    ]);
    return { me, sessions };
  });

  const providers: Record<string, string> = { github: 'GitHub', google: 'Google', email: 'Email link' };
  const clients = { web: 'Web dashboard', ios: 'iPhone app', desktop: 'Orbit Setup' };
  const themes: [Theme, string][] = [['system', 'Same as system'], ['light', 'Light'], ['dark', 'Dark']];

  let theme = $state(savedTheme());
  let error = $state('');
  let ending = $state.raw<Session | undefined>();
  let deleting = $state(false);

  async function leave() {
    clearCache();
    await goto('/login');
  }

  async function signOut() {
    error = '';
    try {
      await api('POST', '/api/auth/logout');
      await leave();
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    }
  }

  async function endSession(session: Session) {
    await api('DELETE', `/api/me/sessions/${session.id}`);
    await account.reload();
  }

  async function deleteAccount() {
    const { confirmation_token } = await api<{ confirmation_token: string }>('POST', '/api/me/delete-intent');
    await api('DELETE', '/api/me', { confirmation_token });
    await leave();
  }
</script>

<svelte:head><title>Settings · Orbit</title></svelte:head>

<div class="sections">
  <h1>Settings</h1>
  <PushSettings />

  <fieldset class="card">
    <legend>Appearance</legend>
    {#each themes as [value, label] (value)}
      <label class="choice">
        <input type="radio" name="theme" {value} bind:group={theme} onchange={() => applyTheme(theme)} />
        {label}
      </label>
    {/each}
  </fieldset>

  <Loaded of={account}>
    {#snippet children({ me, sessions })}
      <div class="sections">
        <section class="card stack" aria-labelledby="account">
          <h2 id="account">Account</h2>
          <dl>
            <dt>Email</dt>
            <dd>{me.email}</dd>
            <dt>Sign-in</dt>
            <dd>{me.identities.map((p) => providers[p] ?? p).join(', ')}</dd>
            <dt>Devices</dt>
            <dd>Up to {me.limits.max_devices}</dd>
          </dl>
          <div>
            <button class="btn" type="button" onclick={signOut}>
              <LogOut size={20} strokeWidth={1.75} aria-hidden="true" /> Sign out
            </button>
          </div>
          {#if error}<p class="danger-text" role="alert">{error}</p>{/if}
        </section>

        <section class="card stack" aria-labelledby="sessions">
          <h2 id="sessions">Signed-in sessions</h2>
          <ul class="list">
            {#each sessions as session (session.id)}
              <li class="row between">
                <div>
                  <h3>{clients[session.client]}{session.current ? ' (this browser)' : ''}</h3>
                  <p class="caption muted">Last used {dateTime(session.last_used_at)}</p>
                </div>
                {#if !session.current}
                  <button class="btn" type="button" onclick={() => (ending = session)}>
                    Sign out<span class="sr-only"> {clients[session.client]}, last used {dateTime(session.last_used_at)}</span>
                  </button>
                {/if}
              </li>
            {/each}
          </ul>
        </section>

        <section class="card stack" aria-labelledby="delete">
          <h2 id="delete">Delete account</h2>
          <p class="muted">Removes your devices, history, sources and alerts from this server. It can't be undone.</p>
          <div>
            <button class="btn btn-danger" type="button" onclick={() => (deleting = true)}>
              <Trash2 size={20} strokeWidth={1.75} aria-hidden="true" /> Delete account
            </button>
          </div>
        </section>
      </div>

      {#if deleting}
        <ConfirmDialog
          title="Delete {me.email}?"
          text="All of this account's devices, history, sources and alerts are removed for good. Apps on your iPhone stay until they expire."
          confirmLabel="Delete account"
          onconfirm={deleteAccount}
          onclose={() => (deleting = false)}
        />
      {/if}
    {/snippet}
  </Loaded>

  <section class="card stack" aria-labelledby="about">
    <h2 id="about">About Orbit</h2>
    <p>Free software under the <a href="https://www.gnu.org/licenses/agpl-3.0.html">AGPL-3.0</a>. <a href="https://github.com/ahmadteeb/sideorbit">Source code</a>.</p>
    <p class="muted">
      Builds on <a href="https://github.com/SideStore/SideStore">SideStore</a>,
      <a href="https://github.com/nab138/iloader">iloader</a>,
      <a href="https://github.com/Dadoum/anisette-v3-server">anisette-v3-server</a> and
      <a href="https://github.com/LiveContainer/LiveContainer">LiveContainer</a>.
    </p>
    <p class="caption muted">
      Fonts under the SIL Open Font License:
      <a href="/licenses/ibm-plex-sans-OFL.txt" target="_blank" rel="noopener">IBM Plex Sans</a>,
      <a href="/licenses/jetbrains-mono-OFL.txt" target="_blank" rel="noopener">JetBrains Mono</a>.
    </p>
  </section>
</div>

{#if ending}
  {@const session = ending}
  <ConfirmDialog
    title="Sign out {clients[session.client]}?"
    text="That {clients[session.client]} session, last used {dateTime(session.last_used_at)}, has to sign in again."
    confirmLabel="Sign it out"
    onconfirm={() => endSession(session)}
    onclose={() => (ending = undefined)}
  />
{/if}

<style>
  fieldset {
    margin: 0;
  }
  legend {
    float: left;
    width: 100%;
    font-size: 1.0625rem;
    font-weight: 600;
  }
  legend + * {
    clear: left;
  }
  .choice {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--target-min);
    font-weight: 400;
  }
</style>

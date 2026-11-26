<!-- Web push. Permission is only requested after the user taps "Enable alerts". -->
<script lang="ts">
  import { ApiError, api, asApiError } from '$lib/api';
  import { disablePush, enablePush, isSubscribed, pushBlocker, type PushBlocker } from '$lib/push';

  let view = $state<'loading' | 'off' | 'on' | 'unconfigured' | NonNullable<PushBlocker>>('loading');
  let busy = $state(false);
  let message = $state('');
  let error = $state('');

  async function check() {
    try {
      await api('GET', '/api/push/vapid-public-key');
    } catch (e) {
      // The routes exist only when the server has push keys.
      if (e instanceof ApiError && e.status === 404) {
        view = 'unconfigured';
        return;
      }
    }
    const blocker = pushBlocker();
    view = blocker ?? ((await isSubscribed()) ? 'on' : 'off');
  }
  void check().catch(() => (view = 'unsupported'));

  async function run(action: () => Promise<string>) {
    busy = true;
    message = '';
    error = '';
    try {
      message = await action();
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      busy = false;
    }
  }

  const enable = () =>
    run(async () => {
      if (await enablePush()) {
        view = 'on';
        return 'Alerts are on for this browser.';
      }
      view = pushBlocker() ?? 'off';
      return '';
    });
  const disable = () =>
    run(async () => {
      await disablePush();
      view = 'off';
      return 'Alerts are off for this browser.';
    });
  const test = () =>
    run(async () => {
      const { sent } = await api<{ sent: number }>('POST', '/api/push/test');
      if (!sent) throw new ApiError(0, 'push', 'The test alert could not be delivered.', 'Turn alerts off and on again.');
      return 'Test alert sent. It should arrive in a few seconds.';
    });
</script>

<section class="card stack" aria-labelledby="alerts">
  <h2 id="alerts">Alerts</h2>
  <p class="muted">Orbit warns you before apps expire and when a device stops checking in.</p>

  {#if view === 'loading'}
    <p class="muted" role="status">Checking this browser…</p>
  {:else if view === 'unconfigured'}
    <p>Alerts aren't set up on this server. Whoever runs it needs to add web push keys.</p>
  {:else if view === 'install-ios'}
    <p>On iPhone and iPad, alerts work only from the Home Screen app:</p>
    <ol>
      <li>Open this page in Safari.</li>
      <li>Tap the Share button, then <strong>Add to Home Screen</strong>.</li>
      <li>Open Orbit from your Home Screen and come back here.</li>
    </ol>
  {:else if view === 'unsupported'}
    <p>This browser can't receive alerts. Try Safari, Chrome, Edge or Firefox.</p>
  {:else if view === 'denied'}
    <p>Notifications are blocked for this site. Allow them in your browser's settings, then reload.</p>
  {:else if view === 'off'}
    <div><button class="btn btn-primary" type="button" onclick={enable} disabled={busy}>{busy ? 'Turning on…' : 'Enable alerts'}</button></div>
  {:else}
    <p>Alerts are on for this browser.</p>
    <div class="row">
      <button class="btn" type="button" onclick={test} disabled={busy}>Send test alert</button>
      <button class="btn" type="button" onclick={disable} disabled={busy}>Turn off alerts</button>
    </div>
  {/if}

  <p role="status">{message}</p>
  {#if error}<p class="danger-text" role="alert">{error}</p>{/if}
</section>

<style>
  ol {
    margin: 0;
    padding-left: var(--space-6);
  }
</style>

<script lang="ts">
  import { ArrowLeft, CircleHelp, Trash2 } from '@lucide/svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { clock } from '$lib/clock.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import EventList from '$lib/components/EventList.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import StatusChip from '$lib/components/StatusChip.svelte';
  import { clearCache, resource } from '$lib/resource.svelte';
  import { ago, dateTime, deviceProblems, freshness } from '$lib/status';
  import type { App, Device, RefreshEvent } from '$lib/types';

  const id = page.params.id as string;
  const detail = resource(`device:${id}`, async () => {
    const { devices } = await api<{ devices: Device[] }>('GET', '/api/devices');
    const device = devices.find((d) => d.id === id) ?? null;
    if (!device) return { device, apps: [], events: [] };
    const [{ apps }, { events }] = await Promise.all([
      api<{ apps: App[] }>('GET', `/api/devices/${id}/apps`),
      api<{ events: RefreshEvent[] }>('GET', `/api/events?device_id=${id}`)
    ]);
    return { device, apps, events: events.slice(0, 20) };
  });

  let confirming = $state(false);

  async function remove() {
    await api('DELETE', `/api/devices/${id}`);
    clearCache();
    await goto('/');
  }
</script>

<svelte:head><title>{detail.data?.device?.name ?? 'Device'} · Orbit</title></svelte:head>

<div class="sections">
  <a class="btn back" href="/"><ArrowLeft size={20} strokeWidth={1.75} aria-hidden="true" /> All devices</a>
  <Loaded of={detail}>
    {#snippet children({ device, apps, events })}
      {#if !device}
        <h1 class="sr-only">Device</h1>
        <Notice icon={CircleHelp} title="That device isn't here" text="It may have been removed.">
          <a class="btn" href="/">Go to Home</a>
        </Notice>
      {:else}
        <header class="stack">
          <h1>{device.name}</h1>
          <p class="muted">Last report {ago(device.last_seen_at, clock.now)}</p>
          {#if deviceProblems(device, clock.now).length}
            <div class="row">
              {#each deviceProblems(device, clock.now) as problem (problem.label)}
                <StatusChip status={problem} />
              {/each}
            </div>
          {/if}
        </header>

        <section class="card stack" aria-labelledby="apps">
          <h2 id="apps">Apps</h2>
          {#if apps.length}
            <ul class="list">
              {#each apps as app (app.bundle_id)}
                <li>
                  <div class="row between">
                    <h3>{app.name}</h3>
                    <StatusChip status={freshness(app.expires_at, clock.now)} />
                  </div>
                  <details>
                    <summary>Details<span class="sr-only"> for {app.name}</span></summary>
                    <dl>
                      <dt>Bundle ID</dt>
                      <dd class="mono">{app.bundle_id}</dd>
                      <dt>Version</dt>
                      <dd class="mono">{app.version ?? 'Unknown'}</dd>
                      <dt>Expires</dt>
                      <dd>{app.expires_at ? dateTime(app.expires_at) : 'Unknown'}</dd>
                      {#if app.source_url}
                        <dt>Source</dt>
                        <dd class="mono">{app.source_url}</dd>
                      {/if}
                    </dl>
                  </details>
                </li>
              {/each}
            </ul>
          {:else}
            <p class="muted">No apps reported yet. Open Orbit on this iPhone.</p>
          {/if}
        </section>

        <section class="card stack" aria-labelledby="history">
          <h2 id="history">Recent history</h2>
          {#if events.length}
            <EventList {events} showDevice={false} />
            <a href="/activity">See all activity</a>
          {:else}
            <p class="muted">Nothing has happened yet.</p>
          {/if}
        </section>

        <section class="card stack" aria-labelledby="about">
          <h2 id="about">This device</h2>
          <dl>
            <dt>Model</dt>
            <dd class="mono">{device.model ?? 'Unknown'}</dd>
            <dt>iOS</dt>
            <dd class="mono">{device.ios_version ?? 'Unknown'}</dd>
            <dt>Apple ID</dt>
            <dd class="mono">{device.apple_label ?? 'Not reported'}</dd>
            <dt>Team ID</dt>
            <dd class="mono">{device.apple_team_id ?? 'Not reported'}</dd>
            <dt>App IDs</dt>
            <dd>{device.app_ids_used_7d === null ? 'Not reported' : `${device.app_ids_used_7d} of 10 used this week`}</dd>
          </dl>
          <div>
            <button class="btn btn-danger" type="button" onclick={() => (confirming = true)}>
              <Trash2 size={20} strokeWidth={1.75} aria-hidden="true" /> Remove device
            </button>
          </div>
        </section>

        {#if confirming}
          <ConfirmDialog
            title="Remove {device.name}?"
            text="Orbit forgets this device, its history and its alerts. Apps already on the phone stay installed."
            confirmLabel="Remove {device.name}"
            onconfirm={remove}
            onclose={() => (confirming = false)}
          />
        {/if}
      {/if}
    {/snippet}
  </Loaded>
</div>

<style>
  .back {
    justify-self: start;
  }
</style>

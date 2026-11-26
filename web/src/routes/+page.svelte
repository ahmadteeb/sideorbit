<script lang="ts">
  import { BadgeCheck, BadgeX, CircleHelp, ClockAlert, Smartphone, TriangleAlert } from '@lucide/svelte';
  import { api } from '$lib/api';
  import { clock } from '$lib/clock.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import StatusChip from '$lib/components/StatusChip.svelte';
  import { resource } from '$lib/resource.svelte';
  import { ago, deviceProblems, freshness, summary } from '$lib/status';
  import type { App, Device } from '$lib/types';

  const overview = resource('overview', async () => {
    const { devices } = await api<{ devices: Device[] }>('GET', '/api/devices');
    return Promise.all(
      devices.map(async (device) => ({
        device,
        apps: (await api<{ apps: App[] }>('GET', `/api/devices/${device.id}/apps`)).apps
      }))
    );
  });

  const icons = {
    fresh: BadgeCheck,
    expiring: ClockAlert,
    expired: BadgeX,
    action: TriangleAlert,
    unknown: CircleHelp
  };
</script>

<svelte:head><title>Home · Orbit</title></svelte:head>

<div class="sections">
  <h1>Your devices</h1>
  <Loaded of={overview}>
    {#snippet children(rows)}
      {#if rows.length === 0}
        <Notice
          icon={Smartphone}
          title="No devices yet"
          text="Install Orbit on your iPhone with Orbit Setup on a computer. It appears here after you sign in on the phone."
        >
          <a class="btn btn-primary" href="https://github.com/ahmadteeb/sideorbit/blob/main/docs/USER_GUIDE.md">
            Read the setup guide
          </a>
        </Notice>
      {:else}
        {@const state = summary(rows.flatMap((r) => r.apps), rows.map((r) => r.device), clock.now)}
        {@const Icon = icons[state.icon]}
        <section class="card hero {state.tone}" aria-live="polite">
          <Icon size={28} strokeWidth={1.75} aria-hidden="true" />
          <div>
            <h2>{state.label}</h2>
            <p class="muted">{state.next}</p>
          </div>
        </section>

        <div class="stack">
          {#each rows as { device, apps } (device.id)}
            <section class="card stack" aria-labelledby="device-{device.id}">
              <div>
                <h2 id="device-{device.id}"><a class="title" href="/devices/{device.id}">{device.name}</a></h2>
                <p class="caption muted">
                  {[device.model, device.ios_version && `iOS ${device.ios_version}`].filter(Boolean).join(' · ')}
                  {device.model || device.ios_version ? ' · ' : ''}Last report {ago(device.last_seen_at, clock.now)}
                </p>
              </div>
              {#if deviceProblems(device, clock.now).length}
                <div class="row">
                  {#each deviceProblems(device, clock.now) as problem (problem.label)}
                    <StatusChip status={problem} />
                  {/each}
                </div>
              {/if}
              {#if apps.length}
                <ul class="list">
                  {#each apps as app (app.bundle_id)}
                    <li class="row between">
                      <span>{app.name} {#if app.version}<span class="mono muted">{app.version}</span>{/if}</span>
                      <StatusChip status={freshness(app.expires_at, clock.now)} />
                    </li>
                  {/each}
                </ul>
              {:else}
                <p class="muted">No apps reported yet. Open Orbit on this iPhone.</p>
              {/if}
            </section>
          {/each}
        </div>
      {/if}
    {/snippet}
  </Loaded>
</div>

<style>
  .hero {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .hero h2 {
    color: var(--color-text);
    font-size: 1.375rem;
    line-height: 1.75rem;
  }
  .hero.primary {
    color: var(--color-primary);
  }
  .hero.warning {
    color: var(--color-warning);
  }
  .hero.danger {
    color: var(--color-danger);
  }
  .title {
    display: inline-flex;
    align-items: center;
    min-height: var(--target-min);
  }
</style>

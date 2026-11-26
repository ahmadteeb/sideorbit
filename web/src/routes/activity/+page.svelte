<script lang="ts">
  import { History } from '@lucide/svelte';
  import { api, asApiError } from '$lib/api';
  import EventList from '$lib/components/EventList.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import RefreshChart from '$lib/components/RefreshChart.svelte';
  import { resource } from '$lib/resource.svelte';
  import type { Day, Device, RefreshEvent } from '$lib/types';

  type Page = { events: RefreshEvent[]; next_cursor: string | null };

  let device = $state('');
  let result = $state('');
  let events = $state.raw<RefreshEvent[] | undefined>();
  let cursor = $state<string | null>(null);
  let error = $state('');
  let busy = $state(false);
  // Only the newest request may write: a slow earlier filter must not win.
  let latest = 0;

  const header = resource('activity', async () => {
    const [{ days }, { devices }] = await Promise.all([
      api<{ days: Day[] }>('GET', '/api/events/summary'),
      api<{ devices: Device[] }>('GET', '/api/devices')
    ]);
    return { days, devices };
  });

  async function load(more = false) {
    const request = ++latest;
    busy = true;
    error = '';
    if (!more) events = undefined;
    const query = new URLSearchParams();
    if (device) query.set('device_id', device);
    if (result) query.set('result', result);
    if (more && cursor) query.set('cursor', cursor);
    try {
      const next = await api<Page>('GET', `/api/events?${query}`);
      if (request !== latest) return;
      events = more ? [...(events ?? []), ...next.events] : next.events;
      cursor = next.next_cursor;
    } catch (e) {
      if (request !== latest) return;
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      if (request === latest) busy = false;
    }
  }
  void load();
</script>

<svelte:head><title>Activity · Orbit</title></svelte:head>

<div class="sections">
  <h1>Activity</h1>
  <Loaded of={header}>
    {#snippet children({ days, devices })}
      <RefreshChart {days} />
      <div class="filters">
        <div>
          <label for="device">Device</label>
          <select id="device" bind:value={device} onchange={() => load()}>
            <option value="">All devices</option>
            {#each devices as d (d.id)}<option value={d.id}>{d.name}</option>{/each}
          </select>
        </div>
        <div>
          <label for="result">Result</label>
          <select id="result" bind:value={result} onchange={() => load()}>
            <option value="">All results</option>
            <option value="success">Succeeded</option>
            <option value="failure">Failed</option>
          </select>
        </div>
      </div>
    {/snippet}
  </Loaded>

  <div class="stack" aria-live="polite" aria-busy={busy}>
    {#if error}
      <div class="card stack" role="alert">
        <p class="danger-text">{error}</p>
        <div><button class="btn" type="button" onclick={() => load()}>Retry</button></div>
      </div>
    {/if}
    {#if events?.length}
      <div class="card"><EventList {events} /></div>
      {#if cursor}
        <button class="btn" type="button" disabled={busy} onclick={() => load(true)}>
          {busy ? 'Loading…' : 'Load more'}
        </button>
      {/if}
    {:else if events}
      <Notice
        icon={History}
        title="Nothing here yet"
        text={device || result ? 'No events match these filters.' : 'Refreshes and installs show up here.'}
      />
    {:else if !error}
      <p class="muted" role="status">Loading…</p>
    {/if}
  </div>
</div>

<style>
  .filters {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: var(--space-3);
    margin-top: var(--space-4);
  }
</style>

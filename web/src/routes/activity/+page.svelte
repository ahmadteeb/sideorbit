<script lang="ts">
  import { History } from '@lucide/svelte';
  import { page } from '$app/state';
  import { api, asApiError } from '$lib/api';
  import EventList from '$lib/components/EventList.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import RefreshChart from '$lib/components/RefreshChart.svelte';
  import { onReturn, resource } from '$lib/resource.svelte';
  import type { Day, Device, RefreshEvent } from '$lib/types';

  type Page = { events: RefreshEvent[]; next_cursor: string | null };

  let device = $state(page.url.searchParams.get('device_id') ?? '');
  let result = $state('');
  let events = $state.raw<RefreshEvent[] | undefined>();
  let cursor = $state<string | null>(null);
  let error = $state('');
  let failedMore = $state(false);
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
      failedMore = more;
    } finally {
      if (request === latest) busy = false;
    }
  }
  // A new filter starts the list over.
  function filter() {
    events = undefined;
    void load();
  }
  void load();
  onReturn(() => void load());
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
          <select id="device" bind:value={device} onchange={filter}>
            <option value="">All devices</option>
            {#each devices as d (d.id)}<option value={d.id}>{d.name}</option>{/each}
          </select>
        </div>
        <div>
          <label for="result">Result</label>
          <select id="result" bind:value={result} onchange={filter}>
            <option value="">All results</option>
            <option value="success">Succeeded</option>
            <option value="failure">Failed</option>
          </select>
        </div>
      </div>
    {/snippet}
  </Loaded>

  <p class="sr-only" role="status">{events && !busy ? `${events.length} events shown` : ''}</p>
  <div class="stack" aria-busy={busy}>
    {#if events?.length}
      <div class="card"><EventList {events} /></div>
    {:else if events && !error}
      <Notice
        icon={History}
        title="Nothing here yet"
        text={device || result ? 'No events match these filters.' : 'Refreshes and installs show up here.'}
      />
    {:else if !error}
      <p class="muted">Loading…</p>
    {/if}
    {#if error}
      <div class="card stack" role="alert">
        <p class="danger-text">{error}</p>
        <div><button class="btn" type="button" onclick={() => load(failedMore)}>Retry</button></div>
      </div>
    {:else if events?.length && cursor}
      <button class="btn" type="button" disabled={busy} onclick={() => load(true)}>
        {busy ? 'Loading…' : 'Load more'}
      </button>
    {/if}
  </div>
</div>

<style>
  .filters {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: var(--space-3);
  }
</style>

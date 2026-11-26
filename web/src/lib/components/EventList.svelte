<script lang="ts">
  import { CircleCheck, CircleX } from '@lucide/svelte';
  import { dateTime, eventTitle, failureText } from '$lib/status';
  import type { RefreshEvent } from '$lib/types';

  let { events, showDevice = true }: { events: RefreshEvent[]; showDevice?: boolean } = $props();
  const triggers: Record<string, string> = {
    background: 'In the background',
    shortcut: 'From a Shortcut',
    manual: 'Started by you',
    launch: 'When Orbit opened',
    setup: 'During setup'
  };
</script>

<ul class="list">
  {#each events as event (event.id)}
    {@const ok = event.result === 'success'}
    <li class="event">
      <span class={ok ? 'ok' : 'danger-text'}>
        {#if ok}
          <CircleCheck size={20} strokeWidth={1.75} aria-hidden="true" />
        {:else}
          <CircleX size={20} strokeWidth={1.75} aria-hidden="true" />
        {/if}
      </span>
      <div>
        <h3>{eventTitle(event)}</h3>
        {#if showDevice}<a class="tap" href="/devices/{event.device_id}">{event.device_name}</a>{/if}
        <p class="caption muted">
          {dateTime(event.occurred_at)}
          {#if event.trigger} · {triggers[event.trigger] ?? event.trigger}{/if}
        </p>
        {#if !ok}
          {@const why = failureText(event.error_code)}
          <p>{why.cause} {why.fix}</p>
          <a class="tap" href="https://github.com/ahmadteeb/sideorbit/blob/main/docs/USER_GUIDE.md#troubleshooting" target="_blank" rel="noopener">
            More help<span class="sr-only"> with: {why.cause}</span>
          </a>
        {/if}
      </div>
    </li>
  {/each}
</ul>

<style>
  .event {
    display: flex;
    gap: var(--space-3);
  }
  .ok {
    color: var(--color-primary);
  }
</style>

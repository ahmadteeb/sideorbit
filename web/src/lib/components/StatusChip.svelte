<script lang="ts">
  import { BadgeCheck, BadgeX, CircleHelp, ClockAlert, TriangleAlert } from '@lucide/svelte';
  import type { Status } from '$lib/status';

  let { status }: { status: Status } = $props();
  const icons = {
    fresh: BadgeCheck,
    expiring: ClockAlert,
    expired: BadgeX,
    action: TriangleAlert,
    unknown: CircleHelp
  };
  const Icon = $derived(icons[status.icon]);
</script>

<span class="chip caption {status.tone}">
  <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
  {status.label}
</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-2);
    border: 1px solid currentColor;
    border-radius: var(--radius-sm);
    font-variant-numeric: tabular-nums;
  }
  .primary {
    color: var(--color-primary);
  }
  .warning {
    color: var(--color-warning);
  }
  .danger {
    color: var(--color-danger);
  }
  .info {
    color: var(--color-info);
  }
  .muted {
    color: var(--color-text-muted);
  }
</style>

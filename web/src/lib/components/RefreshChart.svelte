<!-- Refreshes per day. Failures are hatched as well as coloured, and the numbers
     are available as text and as a table. -->
<script lang="ts">
  import type { Day } from '$lib/types';

  let { days }: { days: Day[] } = $props();

  const HEIGHT = 96;
  const succeeded = $derived(days.reduce((n, d) => n + d.success, 0));
  const failed = $derived(days.reduce((n, d) => n + d.failure, 0));
  const total = $derived(succeeded + failed);
  const max = $derived(Math.max(1, ...days.map((d) => d.success + d.failure)));
  const scale = (n: number) => (n === 0 ? 0 : Math.max(3, (n / max) * HEIGHT));
  const short = (date: string) =>
    new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' }).format(
      new Date(date)
    );
</script>

<section class="card stack" aria-labelledby="chart-title">
  <h2 id="chart-title">Refreshes, last 30 days</h2>
  {#if total === 0}
    <p class="muted">No refreshes yet. They appear here after Orbit on your iPhone runs one.</p>
  {:else}
    <p>
      {succeeded} of {total} succeeded ({Math.floor((succeeded / total) * 100)}%){failed
        ? `, ${failed} failed`
        : ''}.
    </p>
    <ul class="legend list caption" aria-hidden="true">
      <li><span class="swatch success"></span> Succeeded</li>
      <li><span class="swatch failure"></span> Failed</li>
    </ul>
    <figure>
      <span class="caption muted">{max} per day</span>
      <svg width="100%" height={HEIGHT} aria-hidden="true">
        <defs>
          <pattern id="hatch" width="5" height="5" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="5" height="5" class="hatch-base" />
            <rect width="2" height="5" class="hatch-line" />
          </pattern>
        </defs>
        {#each days as day, i (day.date)}
          {@const ok = scale(day.success)}
          {@const bad = scale(day.failure)}
          <g>
            <title>{short(day.date)}: {day.success} succeeded, {day.failure} failed</title>
            <!-- A full-height hit area makes small bars easy to point at. -->
            <rect class="hit" x="{(i * 100) / days.length}%" width="{100 / days.length}%" y="0" height={HEIGHT} />
            <rect class="success" x="{(i * 100) / days.length + 0.4}%" width="{100 / days.length - 0.8}%" y={HEIGHT - ok} height={ok} rx="2" />
            <rect fill="url(#hatch)" x="{(i * 100) / days.length + 0.4}%" width="{100 / days.length - 0.8}%" y={HEIGHT - ok - bad - (ok && bad ? 2 : 0)} height={bad} rx="2" />
          </g>
        {/each}
        <line x1="0" x2="100%" y1={HEIGHT - 0.5} y2={HEIGHT - 0.5} />
      </svg>
      <div class="row between caption muted">
        <span>{short(days[0].date)}</span><span>{short(days[days.length - 1].date)}</span>
      </div>
    </figure>
    <details>
      <summary>Show as a table</summary>
      <div class="scroll">
        <table>
          <thead><tr><th scope="col">Day</th><th scope="col">Succeeded</th><th scope="col">Failed</th></tr></thead>
          <tbody>
            {#each days.filter((d) => d.success + d.failure > 0) as day (day.date)}
              <tr><th scope="row">{short(day.date)}</th><td>{day.success}</td><td>{day.failure}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    </details>
  {/if}
</section>

<style>
  figure {
    margin: 0;
  }
  svg {
    display: block;
    overflow: visible;
  }
  .hit {
    fill: transparent;
  }
  rect.success {
    fill: var(--color-primary);
  }
  .hatch-base {
    fill: var(--color-danger);
  }
  .hatch-line {
    fill: var(--color-surface);
  }
  line {
    stroke: var(--color-border-strong);
  }
  .legend {
    display: flex;
    gap: var(--space-4);
  }
  .legend > li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0;
    border: 0;
  }
  .swatch {
    width: var(--space-3);
    height: var(--space-3);
  }
  .swatch.success {
    background: var(--color-primary);
  }
  .swatch.failure {
    background: repeating-linear-gradient(
      45deg,
      var(--color-danger) 0 3px,
      var(--color-surface) 3px 5px
    );
  }
  .scroll {
    overflow-x: auto;
  }
  table {
    border-collapse: collapse;
    font-variant-numeric: tabular-nums;
  }
  th,
  td {
    padding: var(--space-2) var(--space-4) var(--space-2) 0;
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }
</style>

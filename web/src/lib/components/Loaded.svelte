<!-- Loading, error and loaded states for one resource. -->
<script lang="ts" generics="T">
  import { RefreshCw, TriangleAlert } from '@lucide/svelte';
  import type { Snippet } from 'svelte';
  import type { Resource } from '$lib/resource.svelte';
  import Notice from './Notice.svelte';
  import Skeleton from './Skeleton.svelte';

  let { of, children }: { of: Resource<T>; children: Snippet<[T]> } = $props();
</script>

{#if of.error}
  <Notice icon={TriangleAlert} title={of.error.message} text={of.error.fix} alert>
    <button class="btn" type="button" onclick={() => of.reload()} disabled={of.loading}>
      <RefreshCw size={20} strokeWidth={1.75} aria-hidden="true" /> Retry
    </button>
  </Notice>
{/if}
{#if of.data !== undefined}
  {@render children(of.data)}
{:else if !of.error}
  <Skeleton />
{/if}

<script lang="ts">
  import { page } from '$app/state';
  import { CircleHelp } from '@lucide/svelte';
  import Notice from '$lib/components/Notice.svelte';
</script>

<svelte:head><title>Not found · Orbit</title></svelte:head>

<h1 class="sr-only">Error</h1>
<Notice
  icon={CircleHelp}
  title={page.status === 404 ? "There's no page here" : 'Something went wrong'}
  text="Go back to your devices and try again."
>
  <a class="btn" href="/">Go to Home</a>
</Notice>

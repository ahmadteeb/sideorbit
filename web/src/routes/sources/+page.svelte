<script lang="ts">
  import { Library, Trash2 } from '@lucide/svelte';
  import { api, asApiError } from '$lib/api';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import Loaded from '$lib/components/Loaded.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import { resource } from '$lib/resource.svelte';
  import type { Source, SourcePreview } from '$lib/types';

  const sources = resource('sources', async () => (await api<{ sources: Source[] }>('GET', '/api/sources')).sources);

  let url = $state('');
  let error = $state('');
  let busy = $state(false);
  let preview = $state.raw<SourcePreview | undefined>();
  let removing = $state.raw<Source | undefined>();
  let announcement = $state('');

  function validate(): boolean {
    error = /^https:\/\/\S+\.\S+/.test(url.trim()) ? '' : 'Enter a full address that starts with https://';
    return !error;
  }

  async function check(event: SubmitEvent) {
    event.preventDefault();
    preview = undefined;
    if (!validate()) return;
    busy = true;
    try {
      preview = await api<SourcePreview>('POST', '/api/sources/preview', { url: url.trim() });
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      busy = false;
    }
  }

  async function save() {
    if (!preview) return;
    busy = true;
    try {
      await api('POST', '/api/sources', { url: preview.url });
      announcement = `${preview.name} saved.`;
      preview = undefined;
      url = '';
      await sources.reload();
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      busy = false;
    }
  }

  async function remove(source: Source) {
    await api('DELETE', `/api/sources/${source.id}`);
    announcement = `${source.name ?? source.url} removed.`;
    await sources.reload();
  }
</script>

<svelte:head><title>Sources · Orbit</title></svelte:head>

<div class="sections">
  <h1>App sources</h1>
  <p class="sr-only" role="status">{announcement}</p>

  <form class="card stack" onsubmit={check} novalidate>
    <div>
      <label for="url">Source address</label>
      <p class="caption muted" id="url-hint">A link to AltStore-format source JSON.</p>
      <input
        id="url"
        type="url"
        inputmode="url"
        autocomplete="off"
        bind:value={url}
        onblur={() => url && validate()}
        oninput={() => ((preview = undefined), (error = ''))}
        aria-describedby="url-hint url-error"
        aria-invalid={error ? 'true' : undefined}
      />
      <p class="danger-text field-error" id="url-error" role="alert">{error}</p>
    </div>
    <div>
      <button class="btn" type="submit" disabled={busy}>{busy && !preview ? 'Checking…' : 'Check source'}</button>
    </div>
    {#if preview}
      <div class="stack" role="status">
        <h2>{preview.name}</h2>
        <p class="muted">
          {preview.app_count} {preview.app_count === 1 ? 'app' : 'apps'}{preview.apps.length ? `, including ${preview.apps.join(', ')}` : ''}.
        </p>
        <p class="muted">Only add sources you trust. Orbit doesn't check their apps.</p>
        <div><button class="btn btn-primary" type="button" onclick={save} disabled={busy}>{busy ? 'Saving…' : 'Save source'}</button></div>
      </div>
    {/if}
  </form>

  <Loaded of={sources}>
    {#snippet children(list)}
      {#if list.length}
        <ul class="list card">
          {#each list as source (source.id)}
            <li class="row between">
              <div class="who">
                <h3>{source.name ?? 'Unnamed source'}</h3>
                <p class="mono muted">{source.url}</p>
              </div>
              <button class="btn" type="button" onclick={() => (removing = source)} aria-label="Remove {source.name ?? source.url}">
                <Trash2 size={20} strokeWidth={1.75} aria-hidden="true" />
              </button>
            </li>
          {/each}
        </ul>
      {:else}
        <Notice icon={Library} title="No sources saved" text="Add one above. Orbit on your iPhone can use it." />
      {/if}
    {/snippet}
  </Loaded>
</div>

{#if removing}
  {@const source = removing}
  <ConfirmDialog
    title="Remove {source.name ?? source.url}?"
    text="It disappears from your saved sources. Apps you installed from it stay."
    confirmLabel="Remove source"
    onconfirm={() => remove(source)}
    onclose={() => (removing = undefined)}
  />
{/if}

<style>
  .who {
    flex: 1 1 12rem;
    min-width: 0;
  }
</style>

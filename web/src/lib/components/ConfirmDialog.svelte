<!-- Confirmation for destructive actions. The title names what is removed. -->
<script lang="ts">
  import { asApiError } from '$lib/api';

  let {
    title,
    text,
    confirmLabel,
    onconfirm,
    onclose
  }: {
    title: string;
    text: string;
    confirmLabel: string;
    onconfirm: () => Promise<void>;
    onclose: () => void;
  } = $props();

  let busy = $state(false);
  let error = $state('');
  let element: HTMLDialogElement;

  // Closing through the element (not by unmounting) returns focus to the button
  // that opened it. Its close event then unmounts the component.
  function open(dialog: HTMLDialogElement) {
    element = dialog;
    dialog.showModal();
    return () => dialog.close();
  }

  async function confirm() {
    busy = true;
    error = '';
    try {
      await onconfirm();
      element.close();
    } catch (e) {
      const failure = asApiError(e);
      error = `${failure.message} ${failure.fix}`;
    } finally {
      busy = false;
    }
  }
</script>

<dialog {@attach open} aria-labelledby="confirm-title" {onclose} oncancel={(e) => busy && e.preventDefault()}>
  <div class="stack">
    <h2 id="confirm-title">{title}</h2>
    <p class="muted">{text}</p>
    {#if error}<p class="danger-text" role="alert">{error}</p>{/if}
    <div class="row">
      <!-- Cancel comes first, so it takes focus. -->
      <button class="btn" type="button" onclick={() => element.close()} disabled={busy}>Cancel</button>
      <button class="btn btn-danger" type="button" onclick={confirm} disabled={busy}>
        {busy ? 'Working…' : confirmLabel}
      </button>
    </div>
  </div>
</dialog>

<style>
  dialog {
    width: min(28rem, calc(100vw - 2 * var(--space-4)));
    padding: var(--space-6);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    color: var(--color-text);
  }
  dialog::backdrop {
    background: var(--color-scrim);
  }
</style>

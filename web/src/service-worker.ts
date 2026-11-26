/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// Web push only. API data is never cached offline.
const sw = self as unknown as ServiceWorkerGlobalScope;

sw.addEventListener('push', (event) => {
  let data: { title?: string; body?: string; url?: string } = {};
  try {
    data = event.data?.json() ?? {};
  } catch {
    data = {};
  }
  event.waitUntil(
    sw.registration.showNotification(data.title ?? 'Orbit', {
      body: data.body ?? '',
      icon: '/icons/icon-192.png',
      data: { url: data.url ?? '/' }
    })
  );
});

sw.addEventListener('notificationclick', (event) => {
  event.notification.close();
  // Only ever open a page of this dashboard.
  let target = new URL(event.notification.data?.url ?? '/', sw.location.origin);
  if (target.origin !== sw.location.origin) target = new URL('/', sw.location.origin);
  event.waitUntil(
    (async () => {
      const windows = await sw.clients.matchAll({ type: 'window', includeUncontrolled: true });
      const open = windows.find((w) => 'focus' in w);
      if (open) {
        await open.focus();
        await open.navigate(target.href).catch(() => sw.clients.openWindow(target.href));
        return;
      }
      await sw.clients.openWindow(target.href);
    })()
  );
});

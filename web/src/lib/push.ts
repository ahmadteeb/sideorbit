// Web push: subscribe, unsubscribe and platform checks.
import { ApiError, api } from './api';

/** Why alerts can't be turned on here, or null when they can. */
export type PushBlocker = 'install-ios' | 'unsupported' | 'denied' | null;

export function isIOS(userAgent: string, touchPoints: number): boolean {
  // iPadOS reports itself as a Mac with a touch screen.
  return /iPad|iPhone|iPod/.test(userAgent) || (userAgent.includes('Macintosh') && touchPoints > 1);
}

export function pushBlocker(): PushBlocker {
  const standalone =
    matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true;
  // iOS only allows web push from a Home Screen app.
  if (isIOS(navigator.userAgent, navigator.maxTouchPoints) && !standalone) return 'install-ios';
  if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window))
    return 'unsupported';
  return Notification.permission === 'denied' ? 'denied' : null;
}

export function decodeKey(base64url: string): Uint8Array<ArrayBuffer> {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '='));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

async function current(): Promise<PushSubscription | null> {
  const registration = await navigator.serviceWorker.getRegistration();
  return (await registration?.pushManager.getSubscription()) ?? null;
}

// The server returns the same id for the same endpoint.
async function register(subscription: PushSubscription): Promise<string> {
  return (await api<{ id: string }>('POST', '/api/push/subscriptions', subscription.toJSON())).id;
}

/**
 * Whether this browser gets this account's alerts. An existing subscription is
 * registered again, which moves it here if someone else used the browser.
 */
export async function isSubscribed(): Promise<boolean> {
  if (Notification.permission !== 'granted') return false;
  const subscription = await current();
  if (!subscription) return false;
  await register(subscription);
  return true;
}

// navigator.serviceWorker.ready never settles if registration failed.
function ready(): Promise<ServiceWorkerRegistration> {
  return Promise.race([
    navigator.serviceWorker.ready,
    new Promise<never>((_, reject) =>
      setTimeout(
        () => reject(new ApiError(0, 'no_worker', "This browser didn't start Orbit's alert service.", 'Reload the page and try again.')),
        10_000
      )
    )
  ]);
}

/** Asks for permission and subscribes. Call it from a tap, never on load. */
export async function enablePush(): Promise<boolean> {
  if ((await Notification.requestPermission()) !== 'granted') return false;
  const { public_key } = await api<{ public_key: string }>('GET', '/api/push/vapid-public-key');
  const registration = await ready();
  const subscription =
    (await registration.pushManager.getSubscription()) ??
    (await registration.pushManager
      .subscribe({ userVisibleOnly: true, applicationServerKey: decodeKey(public_key) })
      .catch(() => {
        throw new ApiError(0, 'subscribe', "The browser couldn't turn on alerts.", 'Check your connection and notification settings, then try again.');
      }));
  await register(subscription);
  return true;
}

export async function disablePush(): Promise<void> {
  const subscription = await current();
  if (!subscription) return;
  const id = await register(subscription);
  await api('DELETE', `/api/push/subscriptions/${id}`);
  await subscription.unsubscribe();
}

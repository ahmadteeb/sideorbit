// Web push: subscribe, unsubscribe and platform checks.
import { api } from './api';

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

export async function isSubscribed(): Promise<boolean> {
  return Notification.permission === 'granted' && (await current()) !== null;
}

/** Asks for permission and subscribes. Call it from a tap, never on load. */
export async function enablePush(): Promise<boolean> {
  if ((await Notification.requestPermission()) !== 'granted') return false;
  const { public_key } = await api<{ public_key: string }>('GET', '/api/push/vapid-public-key');
  const registration = await navigator.serviceWorker.ready;
  const subscription =
    (await registration.pushManager.getSubscription()) ??
    (await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: decodeKey(public_key)
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

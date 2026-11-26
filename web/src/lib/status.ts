// The status language from MASTER.md §3. Colour is never the only signal: every
// status has an icon and words.
import type { App, Device, RefreshEvent } from './types';

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

export type Tone = 'primary' | 'warning' | 'danger' | 'info' | 'muted';
export type StatusIcon = 'fresh' | 'expiring' | 'expired' | 'action' | 'unknown';
export interface Status {
  tone: Tone;
  icon: StatusIcon;
  label: string;
}

export function freshness(expiresAt: string | null, now: number): Status {
  if (!expiresAt) return { tone: 'muted', icon: 'unknown', label: 'No expiry reported' };
  const left = Date.parse(expiresAt) - now;
  if (left <= 0) return { tone: 'danger', icon: 'expired', label: 'Expired' };
  if (left <= 48 * HOUR) {
    const label =
      left < HOUR
        ? `Expires in ${Math.max(1, Math.floor(left / 60_000))}m`
        : `Expires in ${Math.floor(left / HOUR)}h`;
    return { tone: 'warning', icon: 'expiring', label };
  }
  return { tone: 'primary', icon: 'fresh', label: `Fresh · ${Math.floor(left / DAY)}d left` };
}

/** Problems on a device that need the user. Empty when everything is fine. */
export function deviceProblems(device: Device, now: number): Status[] {
  const action = (label: string): Status => ({ tone: 'warning', icon: 'action', label });
  const problems: Status[] = [];
  if (device.pairing_ok === false) problems.push(action('Re-pair device'));
  if (device.vpn_ok === false) problems.push(action('Turn on VPN'));
  if (device.last_seen_at && now - Date.parse(device.last_seen_at) > 48 * HOUR)
    problems.push(action("Hasn't checked in"));
  return problems;
}

/** The one-sentence answer to "will my apps expire?". */
export function summary(
  apps: App[],
  devices: Device[],
  now: number
): Status & { next: string } {
  const states = apps.map((a) => freshness(a.expires_at, now).icon);
  const expired = states.filter((s) => s === 'expired').length;
  const expiring = states.filter((s) => s === 'expiring').length;
  const count = (n: number) => (n === 1 ? '1 app' : `${n} apps`);
  if (expired)
    return {
      tone: 'danger',
      icon: 'expired',
      label: `${count(expired)} expired`,
      next: 'Open Orbit on that iPhone and tap Refresh.'
    };
  if (expiring)
    return {
      tone: 'warning',
      icon: 'expiring',
      label: `${count(expiring)} ${expiring === 1 ? 'expires' : 'expire'} soon`,
      next: 'Open Orbit on that iPhone and tap Refresh.'
    };
  if (devices.some((d) => deviceProblems(d, now).length))
    return {
      tone: 'warning',
      icon: 'action',
      label: 'A device needs attention',
      next: 'Your apps are still fresh. Check the device below.'
    };
  if (!apps.length)
    return {
      tone: 'muted',
      icon: 'unknown',
      label: 'No apps reported yet',
      next: 'Open Orbit on your iPhone so it can report its apps.'
    };
  return {
    tone: 'primary',
    icon: 'fresh',
    label: 'Your apps are up to date',
    next: 'Orbit refreshes them before they expire. Nothing to do.'
  };
}

const failures: Record<string, { cause: string; fix: string }> = {
  VPN_OFF: { cause: 'The VPN was off.', fix: 'Turn on LocalDevVPN, then refresh in Orbit.' },
  PAIRING_INVALID: {
    cause: 'The pairing file no longer works.',
    fix: 'Open Orbit on the iPhone and choose Re-pair.'
  },
  APPLE_REAUTH_2FA: {
    cause: 'Apple asked for a verification code.',
    fix: 'Open Orbit on the iPhone and confirm your Apple ID.'
  },
  APP_ID_LIMIT: {
    cause: 'This Apple ID used all 10 App IDs for the week.',
    fix: 'Wait for a slot to free up. Orbit shows the date.'
  },
  ANISETTE_UNAVAILABLE: {
    cause: "Orbit's sign-in helper server didn't respond.",
    fix: 'It retries by itself. If it persists, tell whoever runs your server.'
  }
};

export function failureText(code: string | null): { cause: string; fix: string } {
  return (
    (code && failures[code]) || {
      cause: code ? `It failed with code ${code}.` : 'It failed.',
      fix: 'Open Orbit on the iPhone and try again.'
    }
  );
}

export function eventTitle(event: Pick<RefreshEvent, 'kind' | 'result'>): string {
  const names = { refresh: 'Refresh', install: 'Install', remove: 'Removal', repair: 'Re-pair' };
  return `${names[event.kind]} ${event.result === 'success' ? 'succeeded' : 'failed'}`;
}

export function ago(iso: string | null, now: number): string {
  if (!iso) return 'never';
  const passed = Math.max(0, now - Date.parse(iso));
  const format = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });
  if (passed < HOUR) return format.format(-Math.floor(passed / 60_000), 'minute');
  if (passed < DAY) return format.format(-Math.floor(passed / HOUR), 'hour');
  return format.format(-Math.floor(passed / DAY), 'day');
}

export function dateTime(iso: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(iso)
  );
}

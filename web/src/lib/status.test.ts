import { describe, expect, test } from 'vitest';
import { ago, deviceProblems, failureText, freshness, summary } from './status';
import type { App, Device } from './types';

const now = Date.parse('2026-11-27T12:00:00Z');
const inHours = (h: number) => new Date(now + h * 3_600_000).toISOString();
const app = (hours: number): App => ({
  bundle_id: `a.${hours}`,
  name: 'App',
  version: null,
  expires_at: inHours(hours),
  source_url: null
});
const device = (over: Partial<Device> = {}): Device => ({
  id: 'd',
  name: 'Phone',
  model: null,
  ios_version: null,
  apple_team_id: null,
  apple_label: null,
  app_ids_used_7d: null,
  vpn_ok: true,
  pairing_ok: true,
  last_seen_at: inHours(-1),
  ...over
});

describe('freshness', () => {
  test.each([
    [120, 'primary', 'Fresh · 5d left'],
    [49, 'primary', 'Fresh · 2d left'],
    [48, 'warning', 'Expires in 48h'],
    [31, 'warning', 'Expires in 31h'],
    [0.5, 'warning', 'Expires in 30m'],
    [0, 'danger', 'Expired'],
    [-3, 'danger', 'Expired']
  ])('%s hours left', (hours, tone, label) => {
    expect(freshness(inHours(hours), now)).toMatchObject({ tone, label });
  });
  test('no date', () => {
    expect(freshness(null, now).tone).toBe('muted');
  });
});

describe('summary', () => {
  const noDate = { ...app(1), bundle_id: 'no.date', expires_at: null };
  test.each([
    ['all fresh', [app(100)], [device()], 'Your apps are up to date'],
    ['unknown is not fresh', [app(100), noDate], [device()], 'Expiry not reported for 1 app'],
    ['one expiring', [app(100), app(10)], [device()], '1 app expires soon'],
    ['two expiring', [app(20), app(10)], [device()], '2 apps expire soon'],
    ['expired wins', [app(-1), app(10)], [device()], '1 app expired'],
    ['device problem', [app(100)], [device({ vpn_ok: false })], 'A device needs attention'],
    ['no apps', [], [device()], 'No apps reported yet']
  ])('%s', (_, apps, devices, label) => {
    expect(summary({ d: apps }, devices, now).label).toBe(label);
  });
  test('names the device and leads with its VPN', () => {
    const pad = device({ id: 'p', name: 'Kitchen iPad', vpn_ok: false });
    expect(summary({ d: [app(100)], p: [app(-1)] }, [device(), pad], now).next).toBe(
      'Turn on LocalDevVPN on Kitchen iPad, then tap Refresh in Orbit.'
    );
  });
});

test('device problems', () => {
  expect(deviceProblems(device(), now)).toEqual([]);
  const labels = deviceProblems(
    device({ vpn_ok: false, pairing_ok: false, last_seen_at: inHours(-50) }),
    now
  ).map((p) => p.label);
  expect(labels).toEqual(['Re-pair device', 'Turn on VPN', "Hasn't checked in"]);
});

test('unknown failure codes still get a cause and a fix', () => {
  const text = failureText('SOMETHING_NEW');
  expect(text.cause).toContain('SOMETHING_NEW');
  expect(text.fix).not.toBe('');
});

test('ago', () => {
  expect(ago(inHours(-3), now)).toBe('3 hours ago');
  expect(ago(null, now)).toBe('never');
});

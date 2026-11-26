// A stand-in for the API in docs/API.md, so the dashboard is tested alone.
import type { Page, Route } from '@playwright/test';

const hours = (n: number) => new Date(Date.now() + n * 3_600_000).toISOString();

export interface Mock {
  signedIn: boolean;
  devices: number;
  calls: string[];
}

export async function mockApi(page: Page, options: Partial<Mock> = {}): Promise<Mock> {
  const mock: Mock = { signedIn: true, devices: 2, calls: [], ...options };
  const devices = [
    { id: 'd1', name: "Ada's iPhone", model: 'iPhone16,1', ios_version: '19.1', apple_team_id: 'ABCDE12345', apple_label: 'a***@icloud.com', app_ids_used_7d: 4, vpn_ok: true, pairing_ok: true, last_seen_at: hours(-2) },
    { id: 'd2', name: 'Kitchen iPad', model: 'iPad14,1', ios_version: '18.4', apple_team_id: null, apple_label: null, app_ids_used_7d: null, vpn_ok: false, pairing_ok: true, last_seen_at: hours(-60) }
  ];
  const apps: Record<string, object[]> = {
    d1: [
      { bundle_id: 'dev.orbitstore.Orbit', name: 'Orbit', version: '1.0.0', expires_at: hours(150), source_url: null },
      { bundle_id: 'com.example.player', name: 'Player', version: '2.3.1', expires_at: hours(31), source_url: 'https://example.com/source.json' }
    ],
    d2: [{ bundle_id: 'com.example.recipes', name: 'Recipes', version: '4.0', expires_at: hours(-3), source_url: null }]
  };
  const events = [
    { id: 'e1', device_id: 'd1', device_name: "Ada's iPhone", kind: 'refresh', trigger: 'shortcut', result: 'success', error_code: null, occurred_at: hours(-2) },
    { id: 'e2', device_id: 'd2', device_name: 'Kitchen iPad', kind: 'refresh', trigger: 'background', result: 'failure', error_code: 'VPN_OFF', occurred_at: hours(-60) }
  ];
  const days = Array.from({ length: 30 }, (_, i) => ({
    date: new Date(Date.now() - (29 - i) * 86_400_000).toISOString().slice(0, 10),
    success: i % 7 === 3 ? 0 : 1 + (i % 3),
    failure: i % 9 === 4 ? 1 : 0
  }));

  await page.route(
    (url) => url.pathname.startsWith('/api/'),
    async (route: Route) => {
      const request = route.request();
      const method = request.method();
      const path = new URL(request.url()).pathname;
      mock.calls.push(`${method} ${path}`);
      const json = (body: unknown, status = 200) =>
        route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
      const empty = (status = 204) => route.fulfill({ status });

      if (path === '/api/auth/methods') return json({ github: true, google: true, email: true });
      if (path === '/api/auth/email/start') return empty(202);
      if (path === '/api/auth/token') return mock.signedIn ? json({ expires_in: 900 }) : json({ error: { code: 'invalid_grant', message: 'x' } }, 400);
      if (!mock.signedIn) return json({ error: { code: 'unauthorized', message: 'You are not signed in.' } }, 401);
      if (method !== 'GET' && !request.headers()['x-orbit-csrf']) return json({ error: { code: 'csrf', message: 'Missing header.' } }, 403);

      if (path === '/api/me') return method === 'DELETE' ? empty() : json({ id: 'u1', email: 'ada@example.com', display_name: 'Ada', identities: ['github'], limits: { max_devices: 5 } });
      if (path === '/api/me/sessions') return json({ sessions: [
        { id: 's1', client: 'web', created_at: hours(-1), last_used_at: hours(-1), current: true },
        { id: 's2', client: 'ios', created_at: hours(-200), last_used_at: hours(-5), current: false }
      ] });
      if (path === '/api/devices') return json({ devices: devices.slice(0, mock.devices) });
      const app = path.match(/^\/api\/devices\/(\w+)\/apps$/);
      if (app) return json({ apps: apps[app[1]] ?? [] });
      if (path.startsWith('/api/devices/') && method === 'DELETE') {
        mock.devices -= 1;
        return empty();
      }
      if (path === '/api/events/summary') return json({ days });
      if (path === '/api/events') return json({ events, next_cursor: null });
      if (path === '/api/sources' && method === 'GET') return json({ sources: [{ id: 'src1', url: 'https://example.com/source.json', name: 'Example Source' }] });
      if (path === '/api/sources/preview') return json({ url: 'https://apps.example.com/s.json', name: 'Indie Apps', app_count: 3, apps: ['Player', 'Recipes'] });
      if (path === '/api/sources') return json({ id: 'src2' }, 201);
      if (path === '/api/push/vapid-public-key') return json({ public_key: 'BAAA' });
      if (path === '/api/push/subscriptions') return json({ id: 'sub1' }, 201);
      if (path.startsWith("/api/push/subscriptions/")) return empty();
      if (path === "/api/auth/logout") return empty();
      if (path === '/api/push/test') return json({ sent: 1, failed: 0 });
      return json({ error: { code: 'not_found', message: 'Not mocked.' } }, 404);
    }
  );
  return mock;
}

/** Headless browsers can't subscribe to a real push service, so fake the browser side. */
export async function mockPushBrowser(page: Page) {
  await page.addInitScript(() => {
    let subscription: object | null = null;
    const manager = {
      getSubscription: async () => subscription,
      subscribe: async () =>
        (subscription = {
          toJSON: () => ({ endpoint: 'https://push.example.com/x', keys: { p256dh: 'k', auth: 'a' } }),
          unsubscribe: async () => ((subscription = null), true)
        })
    };
    const registration = { pushManager: manager };
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { ready: Promise.resolve(registration), getRegistration: async () => registration, register: async () => registration, addEventListener() {} }
    });
    Object.assign(window, { PushManager: class {} });
    let permission = 'default';
    Object.defineProperty(window, 'Notification', {
      value: { get permission() { return permission; }, requestPermission: async () => (permission = 'granted') }
    });
  });
}

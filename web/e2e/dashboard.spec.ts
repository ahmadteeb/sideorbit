import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { mockApi, mockPushBrowser } from './mock';

// The page's CSP must not block the app itself.
test.beforeEach(({ page }) => {
  page.on('console', (message) => {
    if (message.text().includes('Content Security Policy')) throw new Error(message.text());
  });
});

test('a signed-out visitor is sent to sign in and can request an email link', async ({ page }) => {
  await mockApi(page, { signedIn: false });
  await page.goto('/');
  await expect(page).toHaveURL('/login');
  await expect(page.getByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github/start');

  await page.getByRole('button', { name: 'Email me a sign-in link' }).click();
  await expect(page.getByText('Enter an email address')).toBeVisible();
  await page.getByLabel('Email').fill('ada@example.com');
  await page.getByRole('button', { name: 'Email me a sign-in link' }).click();
  await expect(page.getByRole('heading', { name: 'Check your inbox' })).toBeVisible();
});

test('sign-in errors are explained', async ({ page }) => {
  await mockApi(page, { signedIn: false });
  await page.goto('/login?error=account_exists');
  await expect(page.getByRole('alert').filter({ hasText: 'already have an Orbit account' })).toBeVisible();
});

test('home lists devices, apps and what needs attention', async ({ page }) => {
  await mockApi(page);
  await page.goto('/');
  await expect(page.getByRole('heading', { name: '1 app expired' })).toBeVisible();
  await expect(page.getByRole('link', { name: "Ada's iPhone" })).toBeVisible();
  await expect(page.getByText(/Fresh · \dd left/)).toBeVisible();
  await expect(page.getByText(/Expires in 3\dh/)).toBeVisible();
  await expect(page.getByText('Turn on VPN')).toBeVisible();
});

test('no devices shows how to set one up', async ({ page }) => {
  await mockApi(page, { devices: 0 });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'No devices yet' })).toBeVisible();
});

test('a failed load offers a retry that works', async ({ page }) => {
  await mockApi(page);
  let fail = true;
  await page.route('**/api/devices', (route) => (fail ? route.abort() : route.fallback()));
  await page.goto('/');
  await expect(page.getByRole('alert')).toContainText("Couldn't reach Orbit.");
  fail = false;
  await page.getByRole('button', { name: 'Retry' }).click();
  await expect(page.getByRole('link', { name: "Ada's iPhone" })).toBeVisible();
});

test('removing a device asks first and names it', async ({ page }) => {
  const mock = await mockApi(page);
  await page.goto('/devices/d1');
  await page.getByRole('button', { name: 'Remove device' }).click();
  const dialog = page.getByRole('dialog', { name: "Remove Ada's iPhone?" });
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  expect(mock.calls).not.toContain('DELETE /api/devices/d1');
  await expect(page.getByRole('button', { name: 'Remove device' })).toBeFocused();

  await page.getByRole('button', { name: 'Remove device' }).click();
  await page.getByRole('button', { name: "Remove Ada's iPhone" }).click();
  await expect(page).toHaveURL('/');
  expect(mock.calls).toContain('DELETE /api/devices/d1');
});

test('activity explains failures with a cause and a fix', async ({ page }) => {
  await mockApi(page);
  await page.goto('/activity');
  await expect(page.getByText('The VPN was off. Turn on LocalDevVPN')).toBeVisible();
  await expect(page.getByText(/of \d+ succeeded/)).toBeVisible();
  await page.getByText('Show as a table').click();
  await expect(page.getByRole('columnheader', { name: 'Failed' })).toBeVisible();
});

test('a source is previewed before it is saved', async ({ page }) => {
  const mock = await mockApi(page);
  await page.goto('/sources');
  await page.getByLabel('Source address').fill('http://apps.example.com/s.json');
  await page.getByRole('button', { name: 'Check source' }).click();
  await expect(page.getByText('starts with https://')).toBeVisible();

  await page.getByLabel('Source address').fill('https://apps.example.com/s.json');
  await page.getByRole('button', { name: 'Check source' }).click();
  await expect(page.getByRole('heading', { name: 'Indie Apps' })).toBeVisible();
  expect(mock.calls).not.toContain('POST /api/sources');
  await page.getByRole('button', { name: 'Save source' }).click();
  await expect.poll(() => mock.calls).toContain('POST /api/sources');
});

test('alerts are enabled only after a tap, then can be tested', async ({ page }) => {
  await mockPushBrowser(page);
  const mock = await mockApi(page);
  await page.goto('/settings');
  const enable = page.getByRole('button', { name: 'Enable alerts' });
  await expect(enable).toBeVisible();
  expect(await page.evaluate(() => Notification.permission)).toBe('default');
  await enable.click();
  await expect(page.getByText('Alerts are on for this browser.')).toBeVisible();
  expect(mock.calls).toContain('POST /api/push/subscriptions');
  await page.getByRole('button', { name: 'Send test alert' }).click();
  await expect(page.getByText('Test alert sent.')).toBeVisible();
  await page.getByRole('button', { name: 'Turn off alerts' }).click();
  await expect.poll(() => mock.calls).toContain('DELETE /api/push/subscriptions/sub1');
});

test('signing out turns this browser\'s alerts off first', async ({ page }) => {
  await mockPushBrowser(page);
  const mock = await mockApi(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: 'Enable alerts' }).click();
  await expect(page.getByRole('button', { name: 'Turn off alerts' })).toBeVisible();
  await page.getByRole('button', { name: 'Sign out', exact: true }).first().click();
  await expect.poll(() => mock.calls).toContain('POST /api/auth/logout');
  expect(mock.calls.indexOf('DELETE /api/push/subscriptions/sub1')).toBeLessThan(mock.calls.indexOf('POST /api/auth/logout'));
});

test.describe('on an iPhone in Safari', () => {
  test.use({ userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Safari/604.1' });
  test('shows how to add to the Home Screen instead of asking for permission', async ({ page }) => {
    await mockApi(page);
    await page.goto('/settings');
    await expect(page.getByText('Add to Home Screen')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Enable alerts' })).toHaveCount(0);
  });
});

test('the theme choice applies and survives a reload', async ({ page }) => {
  await mockApi(page);
  await page.goto('/settings');
  await page.getByLabel('Dark').check();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

const pages: [string, string, (page: Page) => Promise<unknown>][] = [
  ['login', '/login', (p) => p.getByRole('link', { name: 'Continue with GitHub' }).waitFor()],
  ['home', '/', (p) => p.getByRole('link', { name: "Ada's iPhone" }).waitFor()],
  ['device', '/devices/d1', (p) => p.getByRole('heading', { name: 'Recent history' }).waitFor()],
  ['activity', '/activity', (p) => p.getByText('Refresh failed').waitFor()],
  ['sources', '/sources', (p) => p.getByText('Example Source').waitFor()],
  ['settings', '/settings', (p) => p.getByRole('heading', { name: 'Signed-in sessions' }).waitFor()]
];

for (const scheme of ['light', 'dark'] as const) {
  for (const [name, path, ready] of pages) {
    test(`${name} (${scheme}): accessible, no sideways scroll, screenshots`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme, reducedMotion: 'reduce' });
      await mockApi(page, { signedIn: name !== 'login' });
      await page.setViewportSize({ width: 375, height: 812 });
      await page.goto(path);
      await ready(page);

      const { violations } = await new AxeBuilder({ page }).analyze();
      expect(violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')).toEqual([]);

      for (const width of [375, 768, 1440]) {
        await page.setViewportSize({ width, height: 900 });
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
        expect(overflow, `horizontal scroll at ${width}px`).toBeLessThanOrEqual(0);
        await page.screenshot({ path: `test-results/screenshots/${name}-${width}-${scheme}.png`, fullPage: true });
      }
    });
  }
}

test('every control is at least 44px tall on a phone', async ({ page }) => {
  await mockApi(page);
  await page.setViewportSize({ width: 375, height: 812 });
  for (const [, path, ready] of pages.slice(1)) {
    await page.goto(path);
    await ready(page);
    const small = await page.evaluate(() =>
      [...document.querySelectorAll<HTMLElement>('button, .btn, .tap, select, summary, input:not([type=radio]), nav a')]
        .filter((el) => !el.classList.contains('skip') && el.getClientRects().length)
        .filter((el) => el.getBoundingClientRect().height < 43.5)
        .map((el) => el.outerHTML.slice(0, 80))
    );
    expect(small, path).toEqual([]);
  }
});

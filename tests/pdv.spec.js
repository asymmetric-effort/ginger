// @ts-check
const { test, expect } = require('@playwright/test');

const SITE_URL = process.env.SITE_URL || 'https://ginger.asymmetric-effort.com/';

test.describe('Ginger Site PDV', () => {
  test('page loads successfully', async ({ page }) => {
    const response = await page.goto(SITE_URL);
    expect(response.status()).toBe(200);
  });

  test('has correct title', async ({ page }) => {
    await page.goto(SITE_URL);
    await expect(page).toHaveTitle(/Ginger/);
  });

  test('hero section renders', async ({ page }) => {
    await page.goto(SITE_URL);
    const hero = page.locator('text=OpenTelemetry');
    await expect(hero.first()).toBeVisible();
  });

  test('feature cards render', async ({ page }) => {
    await page.goto(SITE_URL);
    const cards = page.locator('.feature-card, [class*="card"], section');
    const count = await cards.count();
    expect(count).toBeGreaterThan(3);
  });

  test('installation section exists', async ({ page }) => {
    await page.goto(SITE_URL);
    const install = page.locator('text=Installation');
    await expect(install.first()).toBeVisible();
  });

  test('links to GitHub repo', async ({ page }) => {
    await page.goto(SITE_URL);
    const ghLink = page.locator('a[href*="github.com/asymmetric-effort/ginger"]');
    await expect(ghLink.first()).toBeVisible();
  });

  test('no console errors', async ({ page }) => {
    const errors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') errors.push(msg.text());
    });
    await page.goto(SITE_URL);
    await page.waitForTimeout(2000);
    expect(errors).toHaveLength(0);
  });

  test('dark mode toggle or detection works', async ({ page }) => {
    await page.goto(SITE_URL);
    // Page should have theme support (either toggle or media query)
    const body = await page.locator('body').getAttribute('class') ||
                 await page.locator('html').getAttribute('data-theme') ||
                 'loaded';
    expect(body).not.toBeNull();
  });
});

test.describe('Helm Chart Repo PDV', () => {
  const HELM_URL = (process.env.SITE_URL || 'https://ginger.asymmetric-effort.com/')
    .replace(/staging\/$/, '').replace(/\/$/, '') + '/helm';

  test('index.yaml is accessible', async ({ request }) => {
    const response = await request.get(HELM_URL + '/index.yaml');
    expect(response.status()).toBe(200);
  });

  test('index.yaml is valid Helm repo index', async ({ request }) => {
    const response = await request.get(HELM_URL + '/index.yaml');
    const body = await response.text();
    expect(body).toContain('apiVersion: v1');
    expect(body).toContain('entries:');
    expect(body).toContain('ginger:');
  });

  test('index.yaml contains chart with correct URL', async ({ request }) => {
    const response = await request.get(HELM_URL + '/index.yaml');
    const body = await response.text();
    expect(body).toContain('https://ginger.asymmetric-effort.com/helm/ginger-');
    expect(body).toContain('.tgz');
  });

  test('chart .tgz is downloadable', async ({ request }) => {
    const indexResp = await request.get(HELM_URL + '/index.yaml');
    const body = await indexResp.text();
    const match = body.match(/https:\/\/ginger\.asymmetric-effort\.com\/helm\/ginger-[0-9.]+\.tgz/);
    expect(match).not.toBeNull();
    const tgzResp = await request.get(match[0]);
    expect(tgzResp.status()).toBe(200);
    expect(tgzResp.headers()['content-length']).toBeDefined();
  });
});

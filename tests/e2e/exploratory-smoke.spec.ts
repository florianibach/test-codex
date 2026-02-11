import { test, expect } from '@playwright/test';

test.describe('MVP-001 mobile flow', () => {
  test.use({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });

  test('AC5: add flow stays within interaction and time budget on mobile', async ({ page }) => {
    let interactionCount = 0;
    const maxInteractions = 2;
    const maxDurationMs = 5000;

    await page.goto('/');

    const startedAt = Date.now();

    interactionCount += 1;
    await page.getByLabel('Titel *').fill('Kaffeemühle');

    interactionCount += 1;
    await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

    await expect(page.getByText('Kaffeemühle')).toBeVisible();
    await expect(page.getByText('Wartet')).toBeVisible();

    const durationMs = Date.now() - startedAt;
    expect(interactionCount, 'The mobile add flow should need at most two interactions.').toBeLessThanOrEqual(maxInteractions);
    expect(durationMs, 'The mobile add flow should complete quickly on touch devices.').toBeLessThanOrEqual(maxDurationMs);
  });
});

test('exploratory smoke suite: navigation, console, and HTTP errors', async ({ page }) => {
  const consoleErrors: string[] = [];
  const httpErrors: string[] = [];

  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text());
    }
  });

  page.on('response', (response) => {
    const status = response.status();
    if (status >= 400) {
      httpErrors.push(`${status} ${response.url()}`);
    }
  });

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Impulse Pause' })).toBeVisible();

  await page.getByRole('link', { name: 'About' }).click();
  await expect(page.getByRole('heading', { name: 'About' })).toBeVisible();

  await page.getByRole('link', { name: 'Zurück' }).click();
  await expect(page).toHaveURL(/\/$/);

  expect(consoleErrors, `Console errors found: ${consoleErrors.join('\n')}`).toEqual([]);
  expect(httpErrors, `HTTP 4xx/5xx responses found: ${httpErrors.join('\n')}`).toEqual([]);
});

test('MVP-001: title-only add flow creates waiting item', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Titel *').fill('Neue Tastatur');
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  await expect(page.getByText('Neue Tastatur')).toBeVisible();
  await expect(page.getByText('Wartet')).toBeVisible();
});

test('MVP-001: empty title shows validation', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Titel *').fill(' ');
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  await expect(page.getByRole('alert')).toContainText('Bitte gib einen Titel ein.');
});

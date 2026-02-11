import { test, expect } from '@playwright/test';

const uniqueTitle = (prefix: string) => `${prefix} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

test.describe('MVP mobile flow', () => {
  test.use({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });

  test('add flow stays within interaction and time budget on mobile', async ({ page }) => {
    let interactionCount = 0;
    const maxInteractions = 3;
    const maxDurationMs = 10_000;

    await page.goto('/');
    const startedAt = Date.now();

    interactionCount += 1;
    await page.getByRole('link', { name: 'Add item' }).click();

    interactionCount += 1;
    await page.getByLabel('Title *').fill('Coffee grinder');

    interactionCount += 1;
    await page.getByRole('button', { name: 'Add to waitlist' }).click();

    await expect(page.getByText('Coffee grinder')).toBeVisible();
    await expect(page.getByText('Waiting')).toBeVisible();

    const durationMs = Date.now() - startedAt;
    expect(interactionCount).toBeLessThanOrEqual(maxInteractions);
    expect(durationMs).toBeLessThanOrEqual(maxDurationMs);
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
  await expect(page.getByRole('heading', { name: 'Waitlist dashboard' })).toBeVisible();

  await page.getByRole('link', { name: 'About' }).click();
  await expect(page.getByRole('heading', { name: 'About' })).toBeVisible();

  await page.getByRole('link', { name: 'Dashboard' }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.getByRole('link', { name: 'Settings' }).click();
  await expect(page.getByRole('heading', { name: 'Profile settings' })).toBeVisible();

  expect(consoleErrors, `Console errors found: ${consoleErrors.join('\n')}`).toEqual([]);
  expect(httpErrors, `HTTP 4xx/5xx responses found: ${httpErrors.join('\n')}`).toEqual([]);
});

test('title-only add flow creates waiting item', async ({ page }) => {
  await page.goto('/items/new');

  const title = uniqueTitle('Keyboard');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const newItemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(newItemRow).toBeVisible();
  await expect(newItemRow.getByText('Waiting')).toBeVisible();
});

test('empty title shows validation', async ({ page }) => {
  await page.goto('/items/new');

  await page.getByLabel('Title *').fill(' ');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await expect(page.getByRole('alert')).toContainText('Please enter a title.');
});

test('custom wait duration validates invalid values', async ({ page }) => {
  await page.goto('/items/new');

  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('0');
  await page.getByLabel('Title *').fill(uniqueTitle('Vinyl'));

  await page.locator('form[action="/items/new"]').evaluate((form) => {
    (form as HTMLFormElement).noValidate = true;
  });

  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await expect(page.getByRole('alert')).toContainText('valid number of custom hours');
});

test('profile validates invalid hourly wage', async ({ page }) => {
  await page.goto('/settings/profile');

  await page.getByLabel('Net hourly wage').fill('0');

  await page.locator('form[action="/settings/profile"]').evaluate((form) => {
    (form as HTMLFormElement).noValidate = true;
  });

  await page.getByRole('button', { name: 'Save profile' }).click();

  await expect(page.getByRole('alert')).toContainText('valid hourly wage');
});

async function waitForItemStatus(page, title: string, status: string) {
  const itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();

  await expect
    .poll(
      async () => {
        await page.reload();
        if (!(await itemRow.isVisible())) {
          return 'missing';
        }

        const badgeText = (await itemRow.locator('.badge').first().textContent()) ?? '';
        return badgeText.trim();
      },
      {
        timeout: 15_000,
        intervals: [300, 500, 1_000],
      },
    )
    .toBe(status);

  return itemRow;
}

test('item auto-promotes to Ready to buy after wait time elapsed', async ({ page }) => {
  await page.goto('/items/new');

  const title = uniqueTitle('Auto-Promotion');
  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('0.0003');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const itemRow = await waitForItemStatus(page, title, 'Ready to buy');
  await expect(itemRow.getByRole('button', { name: 'Mark as bought' })).toBeVisible();
  await expect(itemRow.getByRole('button', { name: 'Mark as skipped' })).toBeVisible();
});

import { test, expect, type Page } from '@playwright/test';

const uniqueTitle = (prefix: string) => `${prefix} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

async function ensureProfileConfigured(page: Page) {
  await page.goto('/settings/profile');
  await page.getByLabel('Net hourly wage').fill('25');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();
}

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

    await ensureProfileConfigured(page);
    await page.goto('/');
    const startedAt = Date.now();

    interactionCount += 1;
    await page.getByRole('main').getByRole('link', { name: 'Add item' }).click();

    interactionCount += 1;
    const title = uniqueTitle('Coffee grinder');
    await page.getByLabel('Title *').fill(title);

    interactionCount += 1;
    await page.getByRole('button', { name: 'Add to waitlist' }).click();

    const row = page.locator('li.list-group-item').filter({ hasText: title }).first();
    await expect(row).toBeVisible();
    await expect(row.getByText('Waiting')).toBeVisible();

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

  await ensureProfileConfigured(page);
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
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const title = uniqueTitle('Keyboard');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const newItemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(newItemRow).toBeVisible();
  await expect(newItemRow.getByText('Waiting')).toBeVisible();
  await expect(newItemRow).not.toContainText('Work hours:');
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

test('reality check shows work hours and updates after wage change', async ({ page }) => {
  const title = uniqueTitle('Reality');

  await page.goto('/settings/profile');
  await page.getByLabel('Net hourly wage').fill('20');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(title);
  await page.getByLabel('Price').fill('100');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  let itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(itemRow).toContainText('Work hours: 5.0 h');

  await page.goto('/settings/profile');
  await page.getByLabel('Net hourly wage').fill('25');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();

  await page.goto('/');
  itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(itemRow).toContainText('Work hours: 4.0 h');
});


test('edit flow updates item and cancel keeps unchanged', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const title = uniqueTitle('Edit me');
  await page.getByLabel('Title *').fill(title);
  await page.getByLabel('Note').fill('before');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const row = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(row).toBeVisible();
  await row.getByRole('link', { name: 'Edit' }).click();

  await expect(page.getByRole('heading', { name: 'Edit item' })).toBeVisible();
  await page.getByLabel('Title *').fill(`${title} updated`);
  await page.getByLabel('Note').fill('after');
  await page.getByRole('button', { name: 'Save changes' }).click();

  let updatedRow = page.locator('li.list-group-item').filter({ hasText: `${title} updated` }).first();
  await expect(updatedRow).toContainText('after');

  await updatedRow.getByRole('link', { name: 'Edit' }).click();
  await page.getByLabel('Title *').fill(`${title} canceled`);
  await page.getByRole('link', { name: 'Cancel' }).click();

  updatedRow = page.locator('li.list-group-item').filter({ hasText: `${title} updated` }).first();
  await expect(updatedRow).toBeVisible();
  await expect(page.locator('li.list-group-item').filter({ hasText: `${title} canceled` })).toHaveCount(0);
});



test('specific date input is only shown when wait time is set to specific date', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const buyAfterInput = page.getByLabel('Buy after');
  await expect(buyAfterInput).toBeHidden();

  await page.getByLabel('Wait time').selectOption('date');
  await expect(buyAfterInput).toBeVisible();

  await page.getByLabel('Wait time').selectOption('custom');
  await expect(buyAfterInput).toBeHidden();
});

test('editing a skipped item with future wait time reopens it as waiting', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const title = uniqueTitle('Skipped reopen');
  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('0.0003');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, title, 'Ready to buy');
  await readyRow.getByRole('button', { name: 'Mark as skipped' }).click();

  const skippedRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(skippedRow.locator('.badge').first()).toHaveText('Skipped');
  await skippedRow.getByRole('link', { name: 'Edit' }).click();

  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('5');
  await page.getByRole('button', { name: 'Save changes' }).click();

  const waitingRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(waitingRow.getByText('Waiting')).toBeVisible();
});

async function waitForItemStatus(page: Page, title: string, status: string) {
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
  await ensureProfileConfigured(page);
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

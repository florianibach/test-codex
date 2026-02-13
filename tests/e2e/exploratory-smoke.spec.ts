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



test('dashboard filter panel is collapsed by default and opens on demand', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/');

  const filterPanel = page.locator('details.mb-3').first();
  await expect(filterPanel).not.toHaveAttribute('open', '');

  await filterPanel.locator('summary').click();
  await expect(filterPanel).toHaveAttribute('open', '');
});

test('dashboard search, tag filter, and price sort work together', async ({ page }) => {
  await ensureProfileConfigured(page);

  const firstTitle = uniqueTitle('R1-004 First');
  const secondTitle = uniqueTitle('R1-004 Second');
  const searchToken = uniqueTitle('r1-004-token');

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(firstTitle);
  await page.getByLabel('Price').fill('200');
  await page.getByLabel('Note').fill(searchToken);
  await page.getByLabel('Tags').fill('tech');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(secondTitle);
  await page.getByLabel('Price').fill('50');
  await page.getByLabel('Note').fill(searchToken);
  await page.getByLabel('Tags').fill('tech');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const filterPanel = page.locator('details.mb-3').first();
  await filterPanel.locator('summary').click();

  await page.getByLabel('Search').fill(searchToken);
  await page.getByLabel('Tag').fill('tech');
  await page.getByLabel('Sort').selectOption('price_asc');
  await page.getByRole('button', { name: 'Apply' }).click();

  await expect(page).toHaveURL(/\/?q=/);
  await expect(page).toHaveURL(/tag=tech/);
  await expect(page).toHaveURL(/sort=price_asc/);

  const rows = page.locator('li.list-group-item');
  await expect(rows).toHaveCount(2);

  const firstRowText = (await rows.nth(0).textContent()) ?? '';
  const secondRowText = (await rows.nth(1).textContent()) ?? '';

  expect(firstRowText).toContain(secondTitle);
  expect(secondRowText).toContain(firstTitle);

  await expect(page.locator('details.mb-3').first()).toHaveAttribute('open', '');
});


test('dashboard search matches title and link fields explicitly', async ({ page }) => {
  await ensureProfileConfigured(page);

  const titleMatch = uniqueTitle('R1-004 title-match');
  const linkMatch = uniqueTitle('r1-004-link');
  const neutralTitle = uniqueTitle('R1-004 neutral');

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(titleMatch);
  await page.getByLabel('Link').fill('https://example.com/no-match');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(neutralTitle);
  await page.getByLabel('Link').fill(`https://example.com/${linkMatch}`);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const filterPanel = page.locator('details.mb-3').first();
  await filterPanel.locator('summary').click();

  await page.getByLabel('Search').fill(titleMatch);
  await page.getByRole('button', { name: 'Apply' }).click();

  await expect(page.locator('li.list-group-item').filter({ hasText: titleMatch })).toHaveCount(1);
  await expect(page.locator('li.list-group-item').filter({ hasText: neutralTitle })).toHaveCount(0);

  await page.getByLabel('Search').fill(linkMatch);
  await page.getByRole('button', { name: 'Apply' }).click();

  await expect(page.locator('li.list-group-item').filter({ hasText: neutralTitle })).toHaveCount(1);
  await expect(page.locator('li.list-group-item').filter({ hasText: titleMatch })).toHaveCount(0);
});

test('dashboard default sorting is next ready to buy (purchase time ascending)', async ({ page }) => {
  await ensureProfileConfigured(page);

  const earlierTitle = uniqueTitle('R1-004 earlier-ready');
  const laterTitle = uniqueTitle('R1-004 later-ready');

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(laterTitle);
  await page.getByLabel('Wait time').selectOption('30d');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(earlierTitle);
  await page.getByLabel('Wait time').selectOption('24h');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await page.goto('/');

  const titles = await page.locator('li.list-group-item p.fw-semibold').allTextContents();
  const earlierIndex = titles.findIndex((txt) => txt.includes(earlierTitle));
  const laterIndex = titles.findIndex((txt) => txt.includes(laterTitle));

  expect(earlierIndex).toBeGreaterThanOrEqual(0);
  expect(laterIndex).toBeGreaterThanOrEqual(0);
  expect(earlierIndex).toBeLessThan(laterIndex);
});

test('dashboard keeps combined search, status filter and sort consistent after reload', async ({ page }) => {
  await ensureProfileConfigured(page);

  const includeTitle = uniqueTitle('R1-004 include');
  const excludeTitle = uniqueTitle('R1-004 exclude');
  const token = uniqueTitle('r1-004-refresh-token');

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(includeTitle);
  await page.getByLabel('Wait time').selectOption('custom');
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByLabel('Note').fill(token);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  await page.goto('/items/new');
  await page.getByLabel('Title *').fill(excludeTitle);
  await page.getByLabel('Wait time').selectOption('7d');
  await page.getByLabel('Note').fill(token);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, includeTitle, 'Ready to buy');
  await expect(readyRow).toBeVisible();

  const filterPanel = page.locator('details.mb-3').first();
  await filterPanel.locator('summary').click();
  await page.getByLabel('Search').fill(token);
  await page.getByLabel('Waiting').uncheck();
  await page.getByLabel('Ready to buy').check();
  await page.getByLabel('Bought').uncheck();
  await page.getByLabel('Skipped').uncheck();
  await page.getByLabel('Sort').selectOption('newest');
  await page.getByRole('button', { name: 'Apply' }).click();

  await expect(page.locator('li.list-group-item').filter({ hasText: includeTitle })).toHaveCount(1);
  await expect(page.locator('li.list-group-item').filter({ hasText: excludeTitle })).toHaveCount(0);

  await page.reload();

  await expect(page.locator('details.mb-3')).toHaveAttribute('open', '');
  await expect(page).toHaveURL(/status=Ready\+to\+buy/);
  await expect(page).not.toHaveURL(/status=Waiting/);
  await expect(page).not.toHaveURL(/status=Bought/);
  await expect(page).not.toHaveURL(/status=Skipped/);
  await expect(page).toHaveURL(/sort=newest/);
  await expect(page.locator('li.list-group-item').filter({ hasText: includeTitle })).toHaveCount(1);
  await expect(page.locator('li.list-group-item').filter({ hasText: excludeTitle })).toHaveCount(0);
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
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
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
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, title, 'Ready to buy');
  await readyRow.getByRole('button', { name: 'Mark as skipped' }).click();

  await page.locator('details.mb-3 summary').click();
  await page.getByLabel('Skipped').check();
  await page.getByRole('button', { name: 'Apply' }).click();

  const skippedRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(skippedRow.locator('.badge').first()).toHaveText('Skipped');
  await skippedRow.getByRole('link', { name: 'Edit' }).click();

  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('5');
  await page.getByRole('button', { name: 'Save changes' }).click();

  const waitingRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(waitingRow.getByText('Waiting')).toBeVisible();
});



test('snooze +24h is only available for ready items and immediately moves the item back to waiting', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const title = uniqueTitle('Snooze');
  await page.getByLabel('Wait time').selectOption('custom');
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, title, 'Ready to buy');
  await expect(readyRow.getByRole('button', { name: 'Snooze +24h' })).toBeVisible();

  const buyAfterBeforeRaw = await readyRow.locator('time.purchase-allowed-at').getAttribute('datetime');
  expect(buyAfterBeforeRaw).not.toBeNull();

  await readyRow.getByRole('button', { name: 'Snooze +24h' }).click();

  const waitingRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(waitingRow.getByText('Waiting')).toBeVisible();
  await expect(waitingRow.getByRole('button', { name: 'Snooze +24h' })).toHaveCount(0);
  await expect(waitingRow.getByRole('button', { name: 'Mark as bought' })).toHaveCount(0);
  await expect(waitingRow.getByRole('button', { name: 'Mark as skipped' })).toHaveCount(0);

  const buyAfterAfterRaw = await waitingRow.locator('time.purchase-allowed-at').getAttribute('datetime');
  expect(buyAfterAfterRaw).not.toBeNull();

  const buyAfterBefore = new Date(buyAfterBeforeRaw as string);
  const buyAfterAfter = new Date(buyAfterAfterRaw as string);
  expect(buyAfterAfter.getTime()).toBeGreaterThan(buyAfterBefore.getTime() + 23 * 60 * 60 * 1000);
});

async function readInsightsMetrics(page: Page) {
  const metricsSection = page.locator('section.card').filter({ hasText: /Skipped items|No data yet\./ }).first();
  const metricCards = metricsSection.locator('article.metric-card');

  if ((await metricCards.count()) === 0) {
    return { skipped: 0, saved: 0 };
  }

  const skippedRaw = (await metricCards.filter({ hasText: 'Skipped items' }).locator('p.h3').textContent()) ?? '0';
  const savedRaw = (await metricCards.filter({ hasText: 'Saved total' }).locator('p.h3').textContent()) ?? '0';

  const skipped = Number.parseInt(skippedRaw.trim(), 10);
  const saved = Number.parseFloat(savedRaw.trim());

  return {
    skipped: Number.isNaN(skipped) ? 0 : skipped,
    saved: Number.isNaN(saved) ? 0 : saved,
  };
}


test('delete flow supports cancel and removes item from dashboard and insights on confirm', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/insights');
  const beforeDeleteMetrics = await readInsightsMetrics(page);

  await page.goto('/items/new');
  const title = uniqueTitle('Delete me');
  await page.getByLabel('Title *').fill(title);
  await page.getByLabel('Price').fill('99.50');
  await page.getByLabel('Wait time').selectOption('custom');
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, title, 'Ready to buy');
  await readyRow.getByRole('button', { name: 'Mark as skipped' }).click();

  await page.locator('details.mb-3 summary').click();
  await page.getByLabel('Skipped').check();
  await page.getByRole('button', { name: 'Apply' }).click();

  const row = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(row.locator('.badge').first()).toHaveText('Skipped');

  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Delete this item permanently?');
    await dialog.dismiss();
  });
  await row.getByRole('button', { name: 'Delete' }).click();
  await expect(row).toBeVisible();

  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Delete this item permanently?');
    await dialog.accept();
  });
  await row.getByRole('button', { name: 'Delete' }).click();
  await expect(page.locator('li.list-group-item').filter({ hasText: title })).toHaveCount(0);

  await page.goto('/insights');
  const afterDeleteMetrics = await readInsightsMetrics(page);
  expect(afterDeleteMetrics.skipped).toBe(beforeDeleteMetrics.skipped);
  expect(afterDeleteMetrics.saved).toBeCloseTo(beforeDeleteMetrics.saved, 2);
});




async function waitForItemStatus(page: Page, title: string, status: string) {
  await page.goto('/');

  await expect
    .poll(
      async () => {
        await page.goto('/');
        const itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
        if (!(await itemRow.isVisible())) {
          return 'missing';
        }

        const badgeText = (await itemRow.locator('.badge').first().textContent()) ?? '';
        return badgeText.trim();
      },
      {
        timeout: 30_000,
        intervals: [500, 1_000, 1_500],
      },
    )
    .toBe(status);

  return page.locator('li.list-group-item').filter({ hasText: title }).first();
}

test('item auto-promotes to Ready to buy after wait time elapsed', async ({ page }) => {
  await ensureProfileConfigured(page);
  await page.goto('/items/new');

  const title = uniqueTitle('Auto-Promotion');
  await page.getByLabel('Wait time').selectOption('custom');
  await expect(page.getByLabel('Custom hours')).toBeEnabled();
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByLabel('Title *').fill(title);
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const itemRow = await waitForItemStatus(page, title, 'Ready to buy');
  await expect(itemRow.getByRole('button', { name: 'Mark as bought' })).toBeVisible();
  await expect(itemRow.getByRole('button', { name: 'Mark as skipped' })).toBeVisible();
});

test('R1-008 currency from profile is used in forms, list and insights with euro fallback', async ({ page }) => {
  await ensureProfileConfigured(page);

  await page.goto('/settings/profile');
  await page.getByLabel('Currency').fill('CHF');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();

  await page.goto('/items/new');
  await expect(page.getByText('Currency: CHF')).toBeVisible();

  const title = uniqueTitle('R1-008 currency');
  await page.getByLabel('Title *').fill(title);
  await page.getByLabel('Price').fill('88.50');
  await page.getByLabel('Wait time').selectOption('custom');
  await page.getByLabel('Custom hours').fill('0.002');
  await page.getByRole('button', { name: 'Add to waitlist' }).click();

  const readyRow = await waitForItemStatus(page, title, 'Ready to buy');
  await expect(readyRow).toContainText('CHF 88.50');
  await readyRow.getByRole('button', { name: 'Mark as skipped' }).click();

  await page.goto('/insights');
  await expect(page.locator('article.metric-card').filter({ hasText: 'Saved total' }).locator('p.h3')).toContainText('CHF');

  await page.goto('/settings/profile');
  await page.getByLabel('Currency').fill('');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();
  await expect(page.getByLabel('Currency')).toHaveValue('€');
});

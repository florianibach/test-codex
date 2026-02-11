import { test, expect } from '@playwright/test';

const uniqueTitle = (prefix: string) => `${prefix} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

test.describe('MVP-001 mobile flow', () => {
  test.use({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });

  test('AC5: add flow stays within interaction and time budget on mobile', async ({ page }) => {
    let interactionCount = 0;
    const maxInteractions = 2;
    const maxDurationMs = 10_000;

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

  const title = uniqueTitle('Neue Tastatur');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const newItemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(newItemRow).toBeVisible();
  await expect(newItemRow.getByText('Wartet')).toBeVisible();
});

test('MVP-001: empty title shows validation', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Titel *').fill(' ');
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  await expect(page.getByRole('alert')).toContainText('Bitte gib einen Titel ein.');
});


test('MVP-002: wait presets are available and 7 Tage can be selected', async ({ page }) => {
  await page.goto('/');

  const waitSelect = page.getByLabel('Wartezeit');
  await expect(waitSelect).toBeVisible();
  await expect(waitSelect.locator('option')).toHaveText(['24h', '7 Tage', '30 Tage', 'Custom']);

  await waitSelect.selectOption('7d');
  const title = uniqueTitle('Laufschuhe');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(itemRow).toBeVisible();
  await expect(itemRow.getByText(/Kauf erlaubt ab:/)).toBeVisible();
});

test('MVP-002: custom wait duration accepts positive hours', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Wartezeit').selectOption('custom');
  await page.getByLabel('Custom (Stunden)').fill('12');
  const title = uniqueTitle('Bücherregal');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(itemRow).toBeVisible();
  await expect(itemRow.getByText('Wartet')).toBeVisible();
});

test('MVP-002: custom wait duration validates invalid values', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Wartezeit').selectOption('custom');
  await page.getByLabel('Custom (Stunden)').fill('0');
  await page.getByLabel('Titel *').fill(uniqueTitle('Schallplatte'));

  // Browser number constraints (min/step) can block submit before backend validation.
  // Disable native form validation here to verify server-side custom wait validation.
  await page.locator('form[action="/"]').evaluate((form) => {
    (form as HTMLFormElement).noValidate = true;
  });

  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  await expect(page.getByRole('alert')).toContainText('gültige Anzahl Stunden');
});

test('MVP-002: custom hours field is only visible when preset is custom', async ({ page }) => {
  await page.goto('/');

  const customHoursGroup = page.locator('#custom-hours-group');
  const customHoursInput = page.locator('#wait_custom_hours');

  await expect(customHoursGroup).toBeHidden();
  await expect(customHoursInput).toBeDisabled();

  await page.getByLabel('Wartezeit').selectOption('custom');
  await expect(customHoursGroup).toBeVisible();
  await expect(customHoursInput).toBeEnabled();

  await page.getByLabel('Wartezeit').selectOption('24h');
  await expect(customHoursGroup).toBeHidden();
  await expect(customHoursInput).toBeDisabled();
});

test('MVP-002: purchase-allowed timestamp is rendered in browser locale from datetime attribute', async ({ page }) => {
  await page.goto('/');

  const title = uniqueTitle('Locale Test');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const itemRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  const timeElement = itemRow.locator('time.purchase-allowed-at');

  await expect(timeElement).toBeVisible();

  const rawDatetime = await timeElement.getAttribute('datetime');
  expect(rawDatetime, 'datetime attribute should be present').toBeTruthy();

  const expectedText = await page.evaluate((iso) => {
    if (!iso) {
      return '';
    }

    return new Intl.DateTimeFormat(navigator.language, {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(iso));
  }, rawDatetime);

  await expect(timeElement).toHaveText(expectedText);
});


test('MVP-004: profile hourly wage shows as value after save and can be edited', async ({ page }) => {
  await page.goto('/');

  const hourlyWageInput = page.getByLabel('Netto-Stundenlohn');
  if (!(await hourlyWageInput.isVisible())) {
    await page.getByRole('button', { name: 'Edit' }).click();
  }

  await hourlyWageInput.fill('31.5');
  await page.getByRole('button', { name: 'Profil speichern' }).click();

  await expect(page.getByRole('status')).toContainText('Profil gespeichert.');
  await expect(page.locator('#hourly-wage-value')).toHaveText('31.5');
  await expect(page.getByRole('button', { name: 'Edit' })).toBeVisible();
  await expect(page.locator('#profile-edit-form')).toHaveCount(0);
  await expect(page.locator('#profile-save-btn')).toHaveCount(0);

  await page.getByRole('button', { name: 'Edit' }).click();
  await expect(page.getByLabel('Netto-Stundenlohn')).toHaveValue('31.5');
  await expect(page.getByRole('button', { name: 'Profil speichern' })).toBeVisible();

  await page.reload();
  await expect(page.locator('#hourly-wage-value')).toHaveText('31.5');
  await expect(page.locator('#profile-edit-form')).toHaveCount(0);
  await expect(page.locator('#profile-save-btn')).toHaveCount(0);
});

test('MVP-004: profile validates invalid hourly wage', async ({ page }) => {
  await page.goto('/');

  const editButton = page.getByRole('button', { name: 'Edit' });
  if (await editButton.isVisible()) {
    await editButton.click();
  }

  await page.getByLabel('Netto-Stundenlohn').fill('0');

  // Browser number constraints (min/required) can block submit before backend validation.
  // Disable native form validation here to verify server-side hourly wage validation.
  await page.locator('form[action="/profile"]').evaluate((form) => {
    (form as HTMLFormElement).noValidate = true;
  });

  await page.getByRole('button', { name: 'Profil speichern' }).click();

  await expect(page.getByRole('alert')).toContainText('gültigen Stundenlohn');
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
        message: `Expected status ${status} for item ${title}`,
      },
    )
    .toBe(status);

  return itemRow;
}

test('MVP-003: item auto-promotes to Kauf erlaubt after wait time elapsed', async ({ page }) => {
  await page.goto('/');

  const title = uniqueTitle('Auto-Promotion');
  await page.getByLabel('Wartezeit').selectOption('custom');
  await page.getByLabel('Custom (Stunden)').fill('0.0003');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const itemRow = await waitForItemStatus(page, title, 'Kauf erlaubt');
  await expect(itemRow.getByRole('button', { name: 'Als gekauft markieren' })).toBeVisible();
  await expect(itemRow.getByRole('button', { name: 'Als nicht gekauft markieren' })).toBeVisible();
});

test('MVP-003: manual decision persists as Nicht gekauft across reload', async ({ page }) => {
  await page.goto('/');

  const title = uniqueTitle('Manual-Decision');
  await page.getByLabel('Wartezeit').selectOption('custom');
  await page.getByLabel('Custom (Stunden)').fill('0.0003');
  await page.getByLabel('Titel *').fill(title);
  await page.getByRole('button', { name: 'Zur Warteliste hinzufügen' }).click();

  const itemRow = await waitForItemStatus(page, title, 'Kauf erlaubt');
  await itemRow.getByRole('button', { name: 'Als nicht gekauft markieren' }).click();

  const decidedRow = page.locator('li.list-group-item').filter({ hasText: title }).first();
  await expect(decidedRow.getByText('Nicht gekauft')).toBeVisible();

  await page.reload();
  await expect(decidedRow.getByText('Nicht gekauft')).toBeVisible();
  await expect(decidedRow.getByRole('button', { name: 'Als gekauft markieren' })).toHaveCount(0);
  await expect(decidedRow.getByRole('button', { name: 'Als nicht gekauft markieren' })).toHaveCount(0);
});

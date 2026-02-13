import { test, expect, type Page } from '@playwright/test';

const uniqueTitle = (prefix: string) => `${prefix} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

async function ensureProfileConfigured(page: Page) {
  await page.goto('/settings/profile');
  await page.getByLabel('Net hourly wage').fill('25');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();
}

test('R1-005 monkeyish: random-ish interactions keep insights stable', async ({ page }) => {
  const consoleErrors: string[] = [];
  const httpErrors: string[] = [];

  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text());
    }
  });

  page.on('response', (response) => {
    if (response.status() >= 400) {
      httpErrors.push(`${response.status()} ${response.url()}`);
    }
  });

  await ensureProfileConfigured(page);

  let seed = 17;
  const next = () => {
    seed = (seed * 1103515245 + 12345) % 2147483647;
    return seed;
  };

  const waitPresets = ['24h', '7d', '30d', 'custom'] as const;

  for (let i = 0; i < 6; i += 1) {
    const routeRoll = next() % 3;
    if (routeRoll === 0) {
      await page.goto('/');
    } else if (routeRoll === 1) {
      await page.goto('/insights');
      await expect(page.getByRole('heading', { name: 'Insights' })).toBeVisible();
    } else {
      await page.goto('/items/new');
      await page.getByLabel('Title *').fill(uniqueTitle(`Monkeyish ${i}`));
      const preset = waitPresets[next() % waitPresets.length];
      await page.getByLabel('Wait time').selectOption(preset);
      if (preset === 'custom') {
        await page.getByLabel('Custom hours').fill('1');
      }
      if (next() % 2 === 0) {
        await page.getByLabel('Price').fill(String((next() % 200) + 1));
      }
      if (next() % 2 === 0) {
        await page.getByLabel('Tags').fill(next() % 2 === 0 ? 'tech' : 'home');
      }
      await page.getByRole('button', { name: 'Add to waitlist' }).click();
      await expect(page.getByRole('heading', { name: 'Waitlist dashboard' })).toBeVisible();
    }
  }

  await page.goto('/insights');
  await expect(page.getByRole('heading', { name: 'Insights' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Monthly decision trend' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Saved amount trend' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Top skip ratios by category' })).toBeVisible();
  expect(consoleErrors, `Console errors found: ${consoleErrors.join('\n')}`).toEqual([]);
  expect(httpErrors, `HTTP 4xx/5xx responses found: ${httpErrors.join('\n')}`).toEqual([]);
});

test('R1-008 monkeyish: currency flips keep money rendering stable across app areas', async ({ page }) => {
  const consoleErrors: string[] = [];
  const httpErrors: string[] = [];

  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text());
    }
  });

  page.on('response', (response) => {
    if (response.status() >= 400) {
      httpErrors.push(`${response.status()} ${response.url()}`);
    }
  });

  await ensureProfileConfigured(page);

  const currencies = ['CHF', '$', 'EUR', '€'] as const;
  let seed = 42;
  const next = () => {
    seed = (seed * 1664525 + 1013904223) % 4294967296;
    return seed;
  };

  for (let i = 0; i < 4; i += 1) {
    const selectedCurrency = currencies[next() % currencies.length];

    await page.goto('/settings/profile');
    await page.getByLabel('Currency').fill(selectedCurrency);
    await page.getByRole('button', { name: 'Save profile' }).click();
    await expect(page.getByText('Profile saved.')).toBeVisible();

    await page.goto('/items/new');
    await expect(page.getByText(`Currency: ${selectedCurrency}`)).toBeVisible();

    const title = uniqueTitle(`R1-008 monkeyish ${i}`);
    await page.getByLabel('Title *').fill(title);
    await page.getByLabel('Price').fill(String((next() % 90) + 10));
    await page.getByLabel('Wait time').selectOption('custom');
    await page.getByLabel('Custom hours').fill('0.002');
    await page.getByRole('button', { name: 'Add to waitlist' }).click();

    await page.goto('/');
    const row = page.locator('li.list-group-item').filter({ hasText: title }).first();
    await expect(row).toContainText(selectedCurrency);
  }

  await page.goto('/insights');
  await expect(page.getByRole('heading', { name: 'Insights' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Saved amount trend' })).toBeVisible();

  expect(consoleErrors, `Console errors found: ${consoleErrors.join('\n')}`).toEqual([]);
  expect(httpErrors, `HTTP 4xx/5xx responses found: ${httpErrors.join('\n')}`).toEqual([]);
});

import { test, expect } from '@playwright/test';

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

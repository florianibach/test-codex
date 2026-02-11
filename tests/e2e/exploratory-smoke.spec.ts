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
  await expect(page.getByRole('heading', { name: 'MVP läuft ✅' })).toBeVisible();

  await page.getByRole('link', { name: 'About' }).click();
  await expect(page.getByRole('heading', { name: 'About' })).toBeVisible();

  await page.getByRole('link', { name: 'Zurück' }).click();
  await expect(page).toHaveURL(/\/$/);

  expect(consoleErrors, `Console errors found: ${consoleErrors.join('\n')}`).toEqual([]);
  expect(httpErrors, `HTTP 4xx/5xx responses found: ${httpErrors.join('\n')}`).toEqual([]);
});

import { test, expect, type Page } from '@playwright/test';

const uniqueName = (prefix: string) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

async function saveProfile(page: Page, hourlyWage: string, currency = '€') {
  await page.goto('/settings/profile');
  await page.getByLabel('Net hourly wage').fill(hourlyWage);
  await page.getByLabel('Currency').fill(currency);
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();
}

test('R1-006: new profile redirects to settings with reset defaults', async ({ page }) => {
  const profileA = uniqueName('Alice');
  const profileB = uniqueName('BrandNew');

  await page.goto('/switch-profile');
  await page.getByLabel('Profile name').fill(profileA);
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page).toHaveURL(/\/settings\/profile/);

  await saveProfile(page, '45', 'CHF');

  await page.goto('/settings/profile');
  await page.getByRole('link', { name: 'Switch profile' }).click();
  await expect(page).toHaveURL(/\/switch-profile/);

  await page.getByLabel('Profile name').fill(profileB);
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page).toHaveURL(/\/settings\/profile/);

  await expect(page.getByLabel('Net hourly wage')).toHaveValue('');
  await expect(page.getByLabel('Currency')).toHaveValue('€');
  await expect(page.getByLabel('Default wait time')).toHaveValue('24h');
});

test('R1-006: switch-profile page lists existing profiles as quick actions', async ({ page }) => {
  const profileA = uniqueName('QuickA');
  const profileB = uniqueName('QuickB');

  await page.goto('/switch-profile');
  await page.getByLabel('Profile name').fill(profileA);
  await page.getByRole('button', { name: 'Create' }).click();
  await saveProfile(page, '22', 'EUR');

  await page.goto('/settings/profile');
  await page.getByRole('link', { name: 'Switch profile' }).click();
  await page.getByLabel('Profile name').fill(profileB);
  await page.getByRole('button', { name: 'Create' }).click();
  await saveProfile(page, '33', '$');

  await page.goto('/switch-profile');
  await expect(page.getByText('Existing profiles')).toBeVisible();
  await expect(page.getByLabel('Profile name')).toHaveValue('');

  await page.getByRole('button', { name: profileA }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.goto('/settings/profile');
  await expect(page.getByLabel('Net hourly wage')).toHaveValue('22');
  await expect(page.getByLabel('Currency')).toHaveValue('EUR');
});


test('R1-006: profile name can be renamed in settings', async ({ page }) => {
  const profileA = uniqueName('RenameA');
  const profileB = uniqueName('RenameB');

  await page.goto('/switch-profile');
  await page.getByLabel('Profile name').fill(profileA);
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page).toHaveURL(/\/settings\/profile/);

  await page.getByLabel('Profile name').fill(profileB);
  await page.getByLabel('Net hourly wage').fill('31');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile saved.')).toBeVisible();

  await page.goto('/switch-profile');
  await expect(page.getByRole('button', { name: profileB })).toBeVisible();
  await expect(page.getByRole('button', { name: profileA })).toHaveCount(0);
});

import { expect, type Browser, type Page } from '@playwright/test';

export const PASSWORD = 'Passw0rd!';

export async function loginAs(browser: Browser, userId: string): Promise<Page> {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto('/login');
  await page.getByLabel('ユーザーID').fill(userId);
  await page.getByLabel('パスワード').fill(PASSWORD);
  await page.getByRole('button', { name: 'ログイン' }).click();
  await expect(page.getByTestId('current-user')).toBeVisible();
  return page;
}

export function longText(n: number): string {
  return '顧客訪問のための出張費用。'.repeat(Math.ceil(n / 13)).slice(0, n);
}

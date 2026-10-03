import { expect, test } from '@playwright/test';
import { loginAs, PASSWORD } from './helpers';

test('wrong password is rejected and audited', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto('/login');
  await page.getByLabel('ユーザーID').fill('employee1');
  await page.getByLabel('パスワード').fill('wrong-password');
  await page.getByRole('button', { name: 'ログイン' }).click();
  await expect(page.getByRole('alert')).toHaveText('ユーザーIDまたはパスワードが正しくありません');

  const admin = await loginAs(browser, 'admin1');
  await admin.getByRole('link', { name: '監査ログ' }).click();
  await admin.getByLabel('操作種別').selectOption('LOGIN_FAILURE');
  await expect(admin.locator('table.audit tbody tr').first()).toContainText('employee1');
});

test('employee cannot reach admin screens or admin APIs', async ({ browser }) => {
  const page = await loginAs(browser, 'employee1');
  await expect(page.getByRole('link', { name: 'マスタ管理' })).toHaveCount(0);
  await page.goto('/admin/masters');
  await expect(page.getByRole('alert')).toHaveText('この画面を表示する権限がありません。');

  for (const path of ['/api/users', '/api/admin/audit-logs', '/api/expenses?scope=all']) {
    expect((await page.request.get(path)).status(), path).toBe(403);
  }
  const res = await page.request.post('/api/accounts', { data: { code: 'HACK', name: 'x' } });
  expect(res.status()).toBe(403);
});

test('admin manages masters and changes are audited', async ({ browser }) => {
  const page = await loginAs(browser, 'admin1');
  await page.getByRole('link', { name: 'マスタ管理' }).click();
  await page.getByLabel('科目コード').fill('6200');
  await page.getByLabel('科目名', { exact: true }).fill('E2E研修費');
  await page.getByLabel('上限金額').fill('20000');
  await page.getByRole('button', { name: '追加' }).click();
  await expect(page.getByRole('cell', { name: 'E2E研修費' })).toBeVisible();

  await page.getByRole('tab', { name: 'ユーザー' }).click();
  await page.getByLabel('ユーザーID').fill('e2euser');
  await page.getByLabel('氏名').fill('E2E ユーザー');
  await page.getByLabel('所属部門').selectOption({ label: '開発部' });
  await page.getByLabel('パスワード').fill(PASSWORD);
  await page.getByRole('button', { name: '追加' }).click();
  await expect(page.getByRole('cell', { name: 'e2euser' })).toBeVisible();

  await page.getByRole('link', { name: '監査ログ' }).click();
  await page.getByLabel('操作種別').selectOption('ACCOUNT_CREATE');
  await expect(page.locator('table.audit tbody tr').first()).toContainText('admin1');
  await page.getByRole('button', { name: '改ざん検証' }).click();
  await expect(page.getByRole('status')).toContainText('改ざんは検出されませんでした');

  // New user can log in.
  const newUser = await loginAs(browser, 'e2euser');
  await expect(newUser.getByTestId('current-user')).toContainText('E2E ユーザー');
});

test('admin runs the reminder batch from the UI', async ({ browser }) => {
  const page = await loginAs(browser, 'admin1');
  await page.getByRole('link', { name: '帳票・バッチ' }).click();
  await page.getByRole('button', { name: 'リマインドバッチを実行' }).click();
  // Seed data contains one stale SUBMITTED and one stale APPROVED_BY_MGR claim.
  await expect(page.getByTestId('batch-result')).toContainText('対象 2件');
  await expect(page.getByRole('cell', { name: 'manager1' }).first()).toBeVisible();
});

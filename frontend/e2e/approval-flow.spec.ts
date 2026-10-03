import { expect, test } from '@playwright/test';
import { loginAs, longText } from './helpers';

test.describe.configure({ mode: 'serial' });

let expenseUrl = '';
let expenseNumber = '';

test('employee creates a claim; business rules are enforced on submit', async ({ browser }) => {
  const page = await loginAs(browser, 'employee1');
  await page.getByRole('link', { name: '新規申請' }).first().click();
  await page.getByLabel('件名').fill('E2E 名古屋出張');
  await page.getByLabel('勘定科目1').selectOption({ label: '6110 旅費交通費' });
  await page.getByLabel('金額1').fill('35000');
  await page.getByLabel('摘要1').fill('新幹線代');
  // Instant hint in the UI...
  await expect(page.getByText('摘要を50文字以上入力してください（現在4文字）')).toBeVisible();
  await page.getByRole('button', { name: '明細を追加' }).click();
  await page.getByLabel('金額2').fill('1200');
  await page.getByLabel('摘要2').fill('地下鉄');
  await expect(page.getByTestId('total-amount')).toHaveText('¥36,200');

  // ...and the server rejects the submission (draft is kept).
  await page.getByRole('button', { name: '保存して提出' }).click();
  await expect(page.getByRole('alert')).toContainText('items[0].description');
  await expect(page).toHaveURL(/\/expenses\/\d+\/edit$/);

  await page.getByLabel('摘要1').fill(longText(55));
  await page.getByRole('button', { name: '保存して提出' }).click();
  await expect(page).toHaveURL(/\/expenses\/\d+$/);
  await expect(page.locator('h1 [data-status]')).toHaveText('申請中');
  expenseUrl = new URL(page.url()).pathname;
  expenseNumber = await page.getByTestId('expense-number').innerText();
  expect(expenseNumber).toMatch(/^EXP-\d{4}-\d{6}$/);
  // The applicant can only withdraw now.
  await expect(page.getByTestId('action-panel').getByRole('button')).toHaveText(['取下げ']);
});

test('other department manager cannot act on the claim', async ({ browser }) => {
  const page = await loginAs(browser, 'manager2');
  const res = await page.request.post(`/api${expenseUrl}/approve`, { data: { comment: 'ok' } });
  expect(res.status()).toBe(403);
});

test('manager approves with a mandatory comment', async ({ browser }) => {
  const page = await loginAs(browser, 'manager1');
  await page.getByRole('link', { name: '承認待ち' }).click();
  await page.getByRole('link', { name: expenseNumber }).click();
  const approve = page.getByRole('button', { name: '承認', exact: true });
  await expect(approve).toBeDisabled();
  await page.getByLabel('コメント（承認・差戻し時は必須）').fill('業務上必要と認めます');
  await approve.click();
  await expect(page.getByRole('status')).toHaveText('承認しました。');
  await expect(page.locator('h1 [data-status]')).toHaveText('一次承認済');
  await expect(page.getByTestId('action-panel')).toHaveCount(0);
});

test('admin cannot skip states via the API and settles the claim', async ({ browser }) => {
  const page = await loginAs(browser, 'admin1');
  await page.goto(expenseUrl);
  await page.getByLabel('コメント（承認・差戻し時は必須）').fill('精算確定');
  await page.getByRole('button', { name: '精算確定' }).click();
  await expect(page.locator('h1 [data-status]')).toHaveText('精算確定');

  // SETTLED is terminal: the API refuses further transitions.
  const res = await page.request.post(`/api${expenseUrl}/reject`, { data: { comment: 'x' } });
  expect(res.status()).toBe(409);

  // Printable 経費精算書（控）
  await page.getByRole('link', { name: '経費精算書（印刷）' }).click();
  await expect(page.getByRole('heading', { name: '経費精算書（控）' })).toBeVisible();
  await expect(page.getByTestId('report-number')).toHaveText(expenseNumber);
  await expect(page.locator('.stamp-name')).toHaveText(['山田 太郎', '鈴木 一郎', '田中 経理']);
  const pdf = await page.pdf({ format: 'A4' });
  expect(pdf.byteLength).toBeGreaterThan(1000);
});

test('admin exports settled claims as CSV', async ({ browser }) => {
  const page = await loginAs(browser, 'admin1');
  await page.getByRole('link', { name: '帳票・バッチ' }).click();
  await page.getByLabel('開始日').fill('2020-01-01');
  await page.getByLabel('終了日').fill('2099-12-31');
  const download = page.waitForEvent('download');
  await page.getByRole('link', { name: 'CSVダウンロード' }).click();
  const file = await download;
  const path = await file.path();
  const { readFile } = await import('node:fs/promises');
  const csv = await readFile(path, 'utf8');
  expect(csv).toContain('精算日,申請番号,行番号');
  expect(csv).toContain(expenseNumber);
});

test('manager rejects a claim; REJECTED cannot be settled', async ({ browser }) => {
  const emp = await loginAs(browser, 'employee1');
  const created = await emp.request.post('/api/expenses', {
    data: {
      title: '差戻しテスト',
      items: [{ useDate: '2026-10-01', accountId: 1, amount: 500, description: 'x' }],
    },
  });
  const { id } = (await created.json()) as { id: number };
  expect((await emp.request.post(`/api/expenses/${id}/submit`)).status()).toBe(200);

  const mgr = await loginAs(browser, 'manager1');
  await mgr.goto(`/expenses/${id}`);
  await mgr.getByLabel('コメント（承認・差戻し時は必須）').fill('領収書を添付してください');
  await mgr.getByRole('button', { name: '差戻し' }).click();
  await expect(mgr.locator('h1 [data-status]')).toHaveText('差戻し');

  const adm = await loginAs(browser, 'admin1');
  const res = await adm.request.post(`/api/expenses/${id}/settle`, { data: { comment: 'force' } });
  expect(res.status()).toBe(409);
});

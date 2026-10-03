// Pure helpers for the expense form. The rules here only provide instant
// feedback; the backend re-validates everything and is authoritative.
import type { ExpenseItemInput } from '../api/types';

export const HIGH_AMOUNT_THRESHOLD = 30000;
export const MIN_DESCRIPTION_FOR_HIGH_AMOUNT = 50;

export function totalAmount(items: Pick<ExpenseItemInput, 'amount'>[]): number {
  return items.reduce((sum, it) => sum + (Number.isFinite(it.amount) ? it.amount : 0), 0);
}

export function needsLongDescription(item: Pick<ExpenseItemInput, 'amount'>): boolean {
  return item.amount >= HIGH_AMOUNT_THRESHOLD;
}

export function descriptionLength(text: string): number {
  // Count Unicode code points, matching the backend (utf8.RuneCountInString).
  return Array.from(text.trim()).length;
}

/** Returns hint messages keyed by "items[i].field" for the current form state. */
export function itemHints(items: ExpenseItemInput[]): Record<string, string> {
  const hints: Record<string, string> = {};
  items.forEach((it, i) => {
    if (needsLongDescription(it) && descriptionLength(it.description) < MIN_DESCRIPTION_FOR_HIGH_AMOUNT) {
      hints[`items[${i}].description`] =
        `${HIGH_AMOUNT_THRESHOLD.toLocaleString()}円以上の明細は摘要を${MIN_DESCRIPTION_FOR_HIGH_AMOUNT}文字以上入力してください（現在${descriptionLength(it.description)}文字）`;
    }
  });
  return hints;
}

export function emptyItem(useDate: string, accountId = 0): ExpenseItemInput {
  return { useDate, accountId, amount: 0, description: '', attachmentId: null };
}

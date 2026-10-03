import { describe, expect, it } from 'vitest';
import { descriptionLength, emptyItem, itemHints, needsLongDescription, totalAmount } from './expenseForm';

describe('expenseForm', () => {
  it('sums amounts and ignores non-finite values', () => {
    expect(totalAmount([{ amount: 100 }, { amount: 250 }, { amount: Number.NaN }])).toBe(350);
  });

  it('requires a long description from 30,000 yen', () => {
    expect(needsLongDescription({ amount: 29999 })).toBe(false);
    expect(needsLongDescription({ amount: 30000 })).toBe(true);
  });

  it('counts characters as code points after trimming', () => {
    expect(descriptionLength('  あいう  ')).toBe(3);
    expect(descriptionLength('𠮷野家')).toBe(3);
  });

  it('produces hints only for high-amount lines with short descriptions', () => {
    const items = [
      { ...emptyItem('2026-10-01', 1), amount: 30000, description: 'あ'.repeat(49) },
      { ...emptyItem('2026-10-01', 1), amount: 30000, description: 'あ'.repeat(50) },
      { ...emptyItem('2026-10-01', 1), amount: 1000, description: '' },
    ];
    const hints = itemHints(items);
    expect(Object.keys(hints)).toEqual(['items[0].description']);
    expect(hints['items[0].description']).toContain('現在49文字');
  });
});

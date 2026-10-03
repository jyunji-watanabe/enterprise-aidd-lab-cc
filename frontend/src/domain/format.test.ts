import { describe, expect, it } from 'vitest';
import { formatDate, formatDateTime, formatYen, todayISO } from './format';

describe('format', () => {
  it('formats yen with separators', () => {
    expect(formatYen(1234567)).toBe('¥1,234,567');
  });

  it('formats dates in Asia/Tokyo', () => {
    expect(formatDate('2026-09-30T16:00:00Z')).toBe('2026/10/01');
    expect(formatDateTime('2026-09-30T16:05:00Z')).toBe('2026/10/01 01:05');
    expect(formatDate(null)).toBe('-');
    expect(formatDate('garbage')).toBe('-');
  });

  it('computes today in Asia/Tokyo', () => {
    expect(todayISO(new Date('2026-09-30T15:30:00Z'))).toBe('2026-10-01');
  });
});

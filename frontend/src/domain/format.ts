const yen = new Intl.NumberFormat('ja-JP');

export function formatYen(amount: number): string {
  return `¥${yen.format(amount)}`;
}

const dt = new Intl.DateTimeFormat('ja-JP', {
  timeZone: 'Asia/Tokyo',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

const d = new Intl.DateTimeFormat('ja-JP', {
  timeZone: 'Asia/Tokyo',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '-';
  const v = new Date(iso);
  return Number.isNaN(v.getTime()) ? '-' : dt.format(v);
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '-';
  const v = new Date(iso);
  return Number.isNaN(v.getTime()) ? '-' : d.format(v);
}

/** Today's date in Asia/Tokyo as YYYY-MM-DD. */
export function todayISO(now: Date = new Date()): string {
  return new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Tokyo' }).format(now);
}

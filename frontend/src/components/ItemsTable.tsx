import { api } from '../api/api';
import type { ExpenseItem } from '../api/types';
import { formatYen } from '../domain/format';

export function ItemsTable({
  items,
  total,
  links = true,
}: {
  items: ExpenseItem[];
  total: number;
  links?: boolean;
}) {
  return (
    <table className="table">
      <thead>
        <tr>
          <th>#</th>
          <th>利用日</th>
          <th>勘定科目</th>
          <th>摘要（用途）</th>
          <th>領収書</th>
          <th className="num">金額</th>
        </tr>
      </thead>
      <tbody>
        {items.map((it) => (
          <tr key={it.id}>
            <td>{it.lineNo}</td>
            <td className="nowrap">{it.useDate}</td>
            <td>
              {it.accountCode} {it.accountName}
            </td>
            <td className="pre">{it.description}</td>
            <td>
              {it.attachmentId !== null &&
                (links ? (
                  <a href={api.attachmentUrl(it.attachmentId)}>{it.attachmentName}</a>
                ) : (
                  it.attachmentName
                ))}
            </td>
            <td className="num">{formatYen(it.amount)}</td>
          </tr>
        ))}
      </tbody>
      <tfoot>
        <tr>
          <th colSpan={5} className="num">
            合計金額
          </th>
          <th className="num" data-testid="detail-total">
            {formatYen(total)}
          </th>
        </tr>
      </tfoot>
    </table>
  );
}

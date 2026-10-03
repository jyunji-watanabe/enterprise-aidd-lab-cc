import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { Action, ExpenseDetail } from '../api/types';
import { ApiError } from '../api/client';
import { ActionPanel } from './ActionPanel';

function expense(allowedActions: Action[]): ExpenseDetail {
  return {
    id: 1,
    number: 'EXP-2026-000001',
    applicantId: 'employee1',
    applicantName: '山田 太郎',
    departmentId: 1,
    departmentCode: 'SALES',
    departmentName: '営業部',
    title: 't',
    status: 'SUBMITTED',
    totalAmount: 1000,
    appliedAt: null,
    settledAt: null,
    statusChangedAt: '',
    createdAt: '',
    updatedAt: '',
    items: [],
    history: [],
    allowedActions,
    canEdit: false,
  };
}

describe('ActionPanel', () => {
  it('renders nothing when no actions are allowed', () => {
    const { container } = render(<ActionPanel expense={expense([])} onAction={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders exactly the actions allowed by the server', () => {
    render(<ActionPanel expense={expense(['approve', 'reject'])} onAction={vi.fn()} />);
    expect(screen.getByRole('button', { name: '承認' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '差戻し' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '精算確定' })).not.toBeInTheDocument();
  });

  it('requires a comment before approving', async () => {
    const onAction = vi.fn().mockResolvedValue(undefined);
    render(<ActionPanel expense={expense(['approve', 'reject'])} onAction={onAction} />);
    const approve = screen.getByRole('button', { name: '承認' });
    expect(approve).toBeDisabled();
    await userEvent.type(screen.getByRole('textbox'), '確認しました');
    expect(approve).toBeEnabled();
    await userEvent.click(approve);
    expect(onAction).toHaveBeenCalledWith('approve', '確認しました');
  });

  it('submits without comment and shows server errors', async () => {
    const onAction = vi
      .fn()
      .mockRejectedValue(
        new ApiError(422, '入力内容に誤りがあります', [{ field: 'totalAmount', message: '0円以下' }]),
      );
    render(<ActionPanel expense={expense(['submit'])} onAction={onAction} />);
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: '提出' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('0円以下');
  });
});

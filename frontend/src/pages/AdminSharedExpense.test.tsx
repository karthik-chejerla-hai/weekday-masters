import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AdminSharedExpense from './AdminSharedExpense';
import { api } from '../services/api';
import type { SharedExpensePreview } from '../types';

vi.mock('../services/api', () => ({ api: { listBalances: vi.fn(), previewSharedExpense: vi.fn(), confirmSharedExpense: vi.fn(), reverseTransaction: vi.fn() } }));

const preview: SharedExpensePreview = {
  input: { description: 'Team dinner', occurred_on: '2026-11-18', total_cents: 8400, payer_user_id: 'u1', split_method: 'equal', participants: [{ user_id: 'u1' }, { user_id: 'u2' }, { user_id: 'u3' }] },
  payer_name: 'Priya',
  fingerprint: 'f'.repeat(64),
  lines: [
    { user_id: 'u2', name: 'Marcus', share_cents: 2800, net_cents: -2800 },
    { user_id: 'u1', name: 'Priya', share_cents: 2800, net_cents: 5600 },
    { user_id: 'u3', name: 'Tom', share_cents: 2800, net_cents: -2800 },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listBalances).mockResolvedValue([{ user_id: 'u1', name: 'Priya' }, { user_id: 'u2', name: 'Marcus' }, { user_id: 'u3', name: 'Tom' }] as never);
  vi.mocked(api.previewSharedExpense).mockResolvedValue(preview);
  vi.mocked(api.confirmSharedExpense).mockResolvedValue({ id: 't1' } as never);
  vi.mocked(api.reverseTransaction).mockResolvedValue({ id: 't2' } as never);
});

async function show() {
  const user = userEvent.setup();
  render(<MemoryRouter><AdminSharedExpense /></MemoryRouter>);
  await screen.findByLabelText('Description');
  return user;
}

async function fill(user: ReturnType<typeof userEvent.setup>, total = '84.00') {
  await user.type(screen.getByLabelText('Description'), 'Team dinner');
  await user.type(screen.getByLabelText('Total ($)'), total);
}

describe('Shared expense form', () => {
  it('selects the paying member as a participant and lets the admin clear them', async () => {
    const user = await show();
    expect(screen.getByRole('checkbox', { name: 'Priya' })).not.toBeChecked();
    await user.selectOptions(screen.getByLabelText('Paid by'), 'u1');
    expect(screen.getByRole('checkbox', { name: 'Priya' })).toBeChecked();

    await user.click(screen.getByRole('checkbox', { name: 'Priya' }));
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    expect(screen.getByRole('checkbox', { name: 'Priya' })).not.toBeChecked();
  });

  it('unticks a payer chosen by mistake, but keeps one the admin ticked', async () => {
    const user = await show();
    await user.selectOptions(screen.getByLabelText('Paid by'), 'u1');
    await user.selectOptions(screen.getByLabelText('Paid by'), 'u2');
    expect(screen.getByRole('checkbox', { name: 'Priya' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Marcus' })).toBeChecked();

    // Tom was ticked by hand before being made the payer, so a later change keeps him.
    await user.click(screen.getByRole('checkbox', { name: 'Tom' }));
    await user.selectOptions(screen.getByLabelText('Paid by'), 'u3');
    await user.selectOptions(screen.getByLabelText('Paid by'), 'bank');
    expect(screen.getByRole('checkbox', { name: 'Tom' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Marcus' })).not.toBeChecked();
  });

  it('retries a failed save with the same request ID and starts a new one after an edit', async () => {
    const user = await show();
    await fill(user);
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });

    // The response is lost: the server may or may not have recorded the expense.
    vi.mocked(api.confirmSharedExpense).mockRejectedValueOnce(new Error('Network Error'));
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('confirm again');
    expect(screen.getByRole('region', { name: 'Expense preview' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    await screen.findByText('Expense recorded');
    const [first, second] = vi.mocked(api.confirmSharedExpense).mock.calls.map(([input]) => input.request_id);
    expect(first).toBeTruthy();
    expect(second).toBe(first);
  });

  it('keeps the request ID through an Edit that changes nothing', async () => {
    const user = await show();
    await fill(user);
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    vi.mocked(api.confirmSharedExpense).mockRejectedValueOnce(new Error('Network Error'));
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    await screen.findByRole('alert');

    // The admin reopens the form to check it, then previews the same bill again.
    await user.click(screen.getByRole('button', { name: 'Edit' }));
    await user.type(screen.getByLabelText('Description'), 'x{Backspace}');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    await screen.findByText('Expense recorded');
    const [first, second] = vi.mocked(api.confirmSharedExpense).mock.calls.map(([input]) => input.request_id);
    expect(first).toBeTruthy();
    expect(second).toBe(first);
  });

  it('uses a new request ID once the bill is edited', async () => {
    const user = await show();
    await fill(user);
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    vi.mocked(api.confirmSharedExpense).mockRejectedValueOnce({ response: { data: { code: 'preview_stale', message: 'The expense changed since the preview. Review it again.' } } });
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Review it again');
    expect(screen.queryByRole('region', { name: 'Expense preview' })).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('Description'), ' again');
    vi.mocked(api.previewSharedExpense).mockResolvedValueOnce({ ...preview, input: { ...preview.input, description: 'Team dinner again' }, fingerprint: 'e'.repeat(64) });
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    await screen.findByText('Expense recorded');
    const [first, second] = vi.mocked(api.confirmSharedExpense).mock.calls.map(([input]) => input.request_id);
    expect(second).not.toBe(first);
  });

  it('locks the form while saving, so an edit cannot hide the result', async () => {
    const user = await show();
    await fill(user);
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });

    let finish: (value: { id: string }) => void = () => {};
    vi.mocked(api.confirmSharedExpense).mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }) as never);
    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    expect(screen.getByLabelText('Description')).toBeDisabled();
    expect(screen.getByRole('checkbox', { name: 'Tom' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Saving…' }));
    finish({ id: 't1' });
    await screen.findByText('Expense recorded');
    expect(api.confirmSharedExpense).toHaveBeenCalledTimes(1);
  });

  it('previews an equal split paid by a member, then confirms the reviewed figures', async () => {
    const user = await show();
    await fill(user);
    await user.selectOptions(screen.getByLabelText('Paid by'), 'u1');
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('checkbox', { name: 'Tom' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));

    const review = await screen.findByRole('region', { name: 'Expense preview' });
    expect(api.previewSharedExpense).toHaveBeenCalledWith(expect.objectContaining({
      description: 'Team dinner', total_cents: 8400, payer_user_id: 'u1', split_method: 'equal',
      participants: [{ user_id: 'u1' }, { user_id: 'u2' }, { user_id: 'u3' }],
    }));
    expect(review).toHaveTextContent('Paid by Priya');
    expect(review).toHaveTextContent('Priya is credited $56.00');
    expect(screen.getAllByText('$28.00')).toHaveLength(3);

    await user.click(screen.getByRole('button', { name: 'Confirm expense' }));
    await screen.findByText('Expense recorded');
    expect(api.confirmSharedExpense).toHaveBeenCalledWith({ ...preview.input, request_id: expect.stringMatching(/^[0-9a-f-]{36}$/), expected_preview: preview.fingerprint });

    // A mistake is undone with a reversing transaction, never an edit.
    await user.click(screen.getByRole('button', { name: 'Reverse this expense' }));
    await screen.findByText('Expense reversed');
    expect(api.reverseTransaction).toHaveBeenCalledWith('t1', 'Reversal of Team dinner');
    expect(screen.queryByRole('button', { name: 'Reverse this expense' })).not.toBeInTheDocument();
  });

  it('sends a null payer when the club bank paid', async () => {
    const user = await show();
    await fill(user, '9');
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    expect(api.previewSharedExpense).toHaveBeenCalledWith(expect.objectContaining({ total_cents: 900, payer_user_id: null, participants: [{ user_id: 'u2' }] }));
  });

  it('requires exact amounts to add up to the total', async () => {
    const user = await show();
    await fill(user);
    await user.selectOptions(screen.getByLabelText('Split'), 'exact');
    await user.click(screen.getByRole('checkbox', { name: 'Priya' }));
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.type(screen.getByLabelText('Amount for Priya'), '50');
    await user.type(screen.getByLabelText('Amount for Marcus'), '30');
    expect(screen.getByText('$4.00 still to assign.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    expect(screen.getByRole('alert')).toHaveTextContent('The amounts must add up to the total.');
    expect(api.previewSharedExpense).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText('Amount for Marcus'));
    await user.type(screen.getByLabelText('Amount for Marcus'), '34.00');
    expect(screen.getByText('The amounts add up to the total.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    expect(api.previewSharedExpense).toHaveBeenCalledWith(expect.objectContaining({
      split_method: 'exact', participants: [{ user_id: 'u1', amount_cents: 5000 }, { user_id: 'u2', amount_cents: 3400 }],
    }));
  });

  it('sends a weight for each person when splitting by shares', async () => {
    const user = await show();
    await fill(user);
    await user.selectOptions(screen.getByLabelText('Split'), 'shares');
    await user.click(screen.getByRole('checkbox', { name: 'Priya' }));
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    expect(screen.getByLabelText('Shares for Marcus')).toHaveValue(1);
    await user.clear(screen.getByLabelText('Shares for Priya'));
    await user.type(screen.getByLabelText('Shares for Priya'), '2');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    expect(api.previewSharedExpense).toHaveBeenCalledWith(expect.objectContaining({
      split_method: 'shares', participants: [{ user_id: 'u1', weight: 2 }, { user_id: 'u2', weight: 1 }],
    }));
  });

  it('discards the preview when the form changes', async () => {
    const user = await show();
    await fill(user);
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    await user.type(screen.getByLabelText('Total ($)'), '1');
    expect(screen.queryByRole('region', { name: 'Expense preview' })).not.toBeInTheDocument();
  });

  it('rejects a malformed total and shows a server refusal', async () => {
    const user = await show();
    await fill(user, '84.005');
    await user.click(screen.getByRole('checkbox', { name: 'Marcus' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Enter the total in dollars and cents');
    expect(api.previewSharedExpense).not.toHaveBeenCalled();

    vi.mocked(api.previewSharedExpense).mockRejectedValue({ response: { data: { code: 'not_settleable', message: 'Confirm club assets before recording expenses.' } } });
    await user.clear(screen.getByLabelText('Total ($)'));
    await user.type(screen.getByLabelText('Total ($)'), '84');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Confirm club assets before recording expenses.');
  });
});

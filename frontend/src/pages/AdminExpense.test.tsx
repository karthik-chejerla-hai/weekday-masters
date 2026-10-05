import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import AdminExpense from './AdminExpense';
import { api } from '../services/api';
import { expenseFixture } from '../components/assistant/fixtures';
import type { ExpenseInput } from '../types';

vi.mock('../services/api', () => ({ api: { getSession: vi.fn(), listBalances: vi.fn(), previewExpense: vi.fn(), confirmExpense: vi.fn() } }));
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getSession).mockResolvedValue({ session: { ...expenseFixture().session, session_date: '2026-10-01', rsvps: [{ user_id: 'u1', status: 'in' }, { user_id: 'u2', status: 'in' }, { user_id: 'u3', status: 'waitlisted' }] } } as never);
  vi.mocked(api.listBalances).mockResolvedValue([{ user_id: 'u1', name: 'Priya' }, { user_id: 'u2', name: 'Marcus' }, { user_id: 'u3', name: 'Tom' }] as never);
  vi.mocked(api.previewExpense).mockResolvedValue(expenseFixture());
  vi.mocked(api.confirmExpense).mockResolvedValue({ id: 'record' });
});
function show(draft?: ExpenseInput) { return render(<MemoryRouter initialEntries={[{ pathname: '/admin/sessions/s1/expense', state: { draft } }]}><Routes><Route path="/admin/sessions/:id/expense" element={<AdminExpense />} /></Routes></MemoryRouter>); }
describe('Expense form', () => {
  it('lets an early leaver pay only for the first two hours', async () => {
    const result = expenseFixture();
    const split = { ...result, input: { ...result.input, extra_participant_ids: ['u1'] } };
    split.settlement.lines[1].in_extra = false;
    split.settlement.lines[0].amount_cents = 7522;
    split.settlement.lines[1].amount_cents = 4111;
    split.settlement.bands.extra!.heads = 1;
    vi.mocked(api.previewExpense).mockResolvedValue(split);
    const user = userEvent.setup(); show();
    await screen.findByLabelText('Hours played');
    await user.selectOptions(screen.getByLabelText('Hours played'), '3');
    expect(screen.getByRole('checkbox', { name: 'Priya, extra hour' })).toBeChecked();
    await user.click(screen.getByRole('checkbox', { name: 'Marcus, extra hour' }));
    await user.type(screen.getByLabelText('Shuttles used'), '8');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('region', { name: 'Expense preview' });
    expect(api.previewExpense).toHaveBeenCalledWith('s1', { total_hours: 3, shuttles_used: 8, participant_ids: ['u1', 'u2'], extra_participant_ids: ['u1'] });
    expect(screen.getByText('2 hours only')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /Confirm expense/ }));
    await screen.findByText('Expense recorded');
    expect(api.confirmExpense).toHaveBeenCalledWith('s1', { ...split.input, expected_preview: split.fingerprint });
  });
  it('uses confirmed RSVPs and asks for actual shuttle use', async () => {
    show();
    expect(await screen.findByRole('checkbox', { name: 'Priya' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Marcus' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Tom' })).not.toBeChecked();
    expect(screen.getByLabelText('Shuttles used')).toHaveValue(null);
    expect(api.previewExpense).not.toHaveBeenCalled();
  });
  it('retains the early departure group when editing an assistant preview', async () => {
    show({ ...expenseFixture().input, extra_participant_ids: ['u1'] });
    expect(await screen.findByRole('checkbox', { name: 'Priya, extra hour' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Marcus, extra hour' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Marcus' })).toBeChecked();
  });
  it('clears stale previews and keeps extra-hour players within the main group', async () => {
    const user = userEvent.setup(); show({ ...expenseFixture().input });
    await screen.findByLabelText('Shuttles used');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('button', { name: /Confirm expense/ });
    await user.click(screen.getByRole('checkbox', { name: 'Marcus, extra hour' }));
    expect(screen.queryByRole('button', { name: /Confirm expense/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: 'Priya' }));
    expect(screen.queryByRole('checkbox', { name: 'Priya, extra hour' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: 'Tom' }));
    expect(screen.getByRole('checkbox', { name: 'Tom, extra hour' })).toBeChecked();
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await waitFor(() => expect(api.previewExpense).toHaveBeenLastCalledWith('s1', { total_hours: 3, shuttles_used: 8, participant_ids: ['u2', 'u3'], extra_participant_ids: ['u3'] }));
  });
  it('requires an extra-hour player for three hours and omits that group for two hours', async () => {
    const user = userEvent.setup(); show({ ...expenseFixture().input, extra_participant_ids: ['u1'] });
    await screen.findByLabelText('Shuttles used');
    await user.click(screen.getByRole('checkbox', { name: 'Priya, extra hour' }));
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Select at least one player for the extra hour');
    expect(api.previewExpense).not.toHaveBeenCalled();
    await user.selectOptions(screen.getByLabelText('Hours played'), '2');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await waitFor(() => expect(api.previewExpense).toHaveBeenCalledWith('s1', { total_hours: 2, shuttles_used: 8, participant_ids: ['u1', 'u2'] }));
  });
  it('previews without saving and confirms exactly the returned input', async () => {
    const user = userEvent.setup(); show();
    await screen.findByLabelText('Hours played');
    await user.selectOptions(screen.getByLabelText('Hours played'), '3');
    await user.type(screen.getByLabelText('Shuttles used'), '8');
    await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    expect(await screen.findByRole('region', { name: 'Expense preview' })).toBeInTheDocument();
    expect(api.confirmExpense).not.toHaveBeenCalled();
    expect(api.previewExpense).toHaveBeenCalledWith('s1', { total_hours: 3, shuttles_used: 8, participant_ids: ['u1', 'u2'], extra_participant_ids: ['u1', 'u2'] });
    await user.click(screen.getByRole('button', { name: /Confirm expense/ }));
    await screen.findByText('Expense recorded');
    expect(api.confirmExpense).toHaveBeenCalledWith('s1', { ...expenseFixture().input, expected_preview: expenseFixture().fingerprint });
  });
  it('removes an old preview as soon as a field changes', async () => {
    const user = userEvent.setup(); show(); await screen.findByLabelText('Shuttles used');
    await user.type(screen.getByLabelText('Shuttles used'), '8'); await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await screen.findByRole('button', { name: /Confirm expense/ });
    await user.clear(screen.getByLabelText('Shuttles used'));
    expect(screen.queryByRole('button', { name: /Confirm expense/ })).not.toBeInTheDocument();
  });
  it('discards a preview response after the inputs have changed', async () => {
    let resolve!: (value: ReturnType<typeof expenseFixture>) => void;
    vi.mocked(api.previewExpense).mockReturnValue(new Promise((done) => { resolve = done; }));
    const user = userEvent.setup(); show(); await screen.findByLabelText('Shuttles used');
    await user.type(screen.getByLabelText('Shuttles used'), '8'); await user.click(screen.getByRole('button', { name: 'Preview expense' }));
    await user.clear(screen.getByLabelText('Shuttles used')); resolve(expenseFixture());
    await waitFor(() => expect(screen.getByRole('button', { name: 'Preview expense' })).toBeEnabled());
    expect(screen.queryByRole('region', { name: 'Expense preview' })).not.toBeInTheDocument();
  });
});

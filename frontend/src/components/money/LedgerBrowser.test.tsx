import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../services/api';
import type { LedgerActivityView, LedgerEntryView } from '../../types';
import LedgerBrowser from './LedgerBrowser';

vi.mock('../../services/api', () => ({ api: { getLedgerActivity: vi.fn() } }));
const entry = (i: number, overrides: Partial<LedgerEntryView> = {}): LedgerActivityView => ({
  id: `e${i}`, occurred_at: '2026-09-22T15:00:00Z', entry: {
  id: `e${i}`, occurred_at: '2026-09-22T15:00:00Z', kind: 'splitwise_import',
  description: `Game - 22/09 (${i})`, category: 'session', source: 'splitwise',
  member_name: 'Karthik', user_id: 'u1', inactive: false, amount_cents: -1300,
  balance_after_cents: 7755, reversed: false, ...overrides,
  },
});
const mock = vi.mocked(api.getLedgerActivity);
beforeEach(() => { vi.resetAllMocks(); });
const renderBrowser = () => render(<LedgerBrowser userId="u1" revision={0} />);

describe('Ledger browser', () => {
  it('defaults to mine and shows title, source, icon, Sydney date and full balance', async () => {
    mock.mockResolvedValue({ items: [entry(1)], total: 1 });
    renderBrowser();
    expect(await screen.findByText('Game - 22/09 (1)')).toBeInTheDocument();
    expect(mock).toHaveBeenCalledWith('mine', false, 50, 0);
    expect(screen.getByRole('switch', { name: 'Show my transactions only' })).toBeChecked();
    expect(screen.getByText('Source: Splitwise')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Session' })).toBeInTheDocument();
    expect(screen.getByText('23 Sep 2026 · Session')).toBeInTheDocument();
    expect(screen.getByText('Balance $77.55')).toBeInTheDocument();
    expect(screen.getByText(/End of history/)).toBeInTheDocument();
  });

  it('combines both filters and shows names and inactive status in all-member view', async () => {
    mock.mockResolvedValueOnce({ items: [entry(1)], total: 1 })
      .mockResolvedValueOnce({ items: [entry(2, { member_name: 'Ram', inactive: true })], total: 1 })
      .mockResolvedValueOnce({ items: [entry(3, { description: 'Bank transfer', category: 'topup', kind: 'player_topup', source: undefined, amount_cents: 5000 })], total: 1 });
    const user = userEvent.setup(); renderBrowser();
    await screen.findByText('Game - 22/09 (1)');
    await user.click(screen.getByRole('switch', { name: 'Show my transactions only' }));
    expect(await screen.findByText('Ram · Inactive')).toBeInTheDocument();
    await user.click(screen.getByRole('switch', { name: 'Top-ups only' }));
    expect(await screen.findByText('Bank transfer')).toBeInTheDocument();
    expect(mock).toHaveBeenLastCalledWith('all', true, 50, 0);
    expect(screen.getByRole('img', { name: 'Top-up' })).toBeInTheDocument();
    expect(screen.queryByText('Source: Splitwise')).not.toBeInTheDocument();
  });

  it('loads 113 entries across three pages and keeps the selected filters', async () => {
    mock.mockImplementation(async (_scope, _topups, _limit, offset = 0) => ({
      items: Array.from({ length: Math.min(50, 113 - offset) }, (_, i) => entry(i + offset)), total: 113,
    }));
    const user = userEvent.setup(); renderBrowser();
    await screen.findByText('50 of 113 entries');
    await user.click(screen.getByRole('button', { name: 'Load older transactions' }));
    await screen.findByText('100 of 113 entries');
    await user.click(screen.getByRole('button', { name: 'Load older transactions' }));
    await screen.findByText('113 of 113 entries · End of history');
    expect(within(screen.getByRole('list', { name: 'Transactions' })).getAllByRole('listitem')).toHaveLength(113);
    expect(mock).toHaveBeenLastCalledWith('mine', false, 50, 100);
    expect(screen.queryByRole('button', { name: 'Load older transactions' })).not.toBeInTheDocument();
  });

  it('retries a failed older page without losing the first page', async () => {
    mock.mockResolvedValueOnce({ items: [entry(1)], total: 2 }).mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce({ items: [entry(2)], total: 2 });
    const user = userEvent.setup(); renderBrowser();
    await screen.findByText('Game - 22/09 (1)');
    await user.click(screen.getByRole('button', { name: 'Load older transactions' }));
    await screen.findByRole('alert');
    expect(screen.getByText('Game - 22/09 (1)')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('Game - 22/09 (2)')).toBeInTheDocument();
    expect(mock).toHaveBeenLastCalledWith('mine', false, 50, 1);
  });

  it('does not duplicate rows or get stuck when a new entry shifts the next page', async () => {
    mock.mockResolvedValueOnce({ items: Array.from({ length: 50 }, (_, i) => entry(i)), total: 51 })
      .mockResolvedValueOnce({ items: [entry(49), entry(50)], total: 52 });
    const user = userEvent.setup(); renderBrowser();
    await screen.findByText('50 of 51 entries');
    await user.click(screen.getByRole('button', { name: 'Load older transactions' }));
    await screen.findByText('51 entries loaded · End of history');
    expect(within(screen.getByRole('list', { name: 'Transactions' })).getAllByRole('listitem')).toHaveLength(51);
    expect(screen.queryByRole('button', { name: 'Load older transactions' })).not.toBeInTheDocument();
  });

  it('retries the initial request and gives a clear empty state', async () => {
    mock.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ items: [], total: 0 });
    const user = userEvent.setup(); renderBrowser();
    await screen.findByRole('alert');
    await user.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('No transactions match these filters.')).toBeInTheDocument();
  });

  it('ignores an older page response after the filter changes', async () => {
    let resolvePage!: (page: { items: LedgerActivityView[]; total: number }) => void;
    mock.mockResolvedValueOnce({ items: [entry(1)], total: 2 })
      .mockImplementationOnce(() => new Promise((resolve) => { resolvePage = resolve; }))
      .mockResolvedValueOnce({ items: [entry(3, { description: 'New filter' })], total: 1 });
    const user = userEvent.setup(); renderBrowser();
    await screen.findByText('Game - 22/09 (1)');
    await user.click(screen.getByRole('button', { name: 'Load older transactions' }));
    await user.click(screen.getByRole('switch', { name: 'Top-ups only' }));
    await screen.findByText('New filter');
    await act(async () => resolvePage({ items: [entry(2)], total: 2 }));
    expect(screen.queryByText('Game - 22/09 (2)')).not.toBeInTheDocument();
    expect(mock).toHaveBeenLastCalledWith('mine', true, 50, 0);
  });

  it('ignores a stale initial response after a scope change', async () => {
    let resolvePage!: (page: { items: LedgerActivityView[]; total: number }) => void;
    mock.mockImplementationOnce(() => new Promise((resolve) => { resolvePage = resolve; }))
      .mockResolvedValueOnce({ items: [entry(3, { description: 'All members' })], total: 1 });
    const user = userEvent.setup(); renderBrowser();
    await user.click(screen.getByRole('switch', { name: 'Show my transactions only' }));
    await screen.findByText('All members');
    await act(async () => resolvePage({ items: [entry(1)], total: 1 }));
    expect(screen.queryByText('Game - 22/09 (1)')).not.toBeInTheDocument();
  });

  it('shows food and reversal icons without a Splitwise badge on native rows', async () => {
    mock.mockResolvedValue({ items: [entry(1, { category: 'food', description: 'Dinner' }), entry(2, { category: 'reversal', description: 'Correction', source: undefined, kind: 'reversal' }), entry(3, { reversed: true })], total: 3 });
    renderBrowser();
    await screen.findByText('Dinner');
    expect(screen.getByRole('img', { name: 'Food and drink' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Reversal' })).toBeInTheDocument();
    expect(screen.getByText('Reversed')).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('Source: Splitwise')).toHaveLength(2));
  });
});

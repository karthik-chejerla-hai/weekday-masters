import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AdminSettlement from './AdminSettlement';
import { api } from '../services/api';
import type { SettlementPreview } from '../types';

vi.mock('../services/api', () => ({
  api: {
    listBalances: vi.fn(),
    previewSettlement: vi.fn(),
    settleSession: vi.fn(),
    recordShuttlePurchase: vi.fn(),
  },
}));

const members = [
  { user_id: 'u1', name: 'Karthik', balance_cents: 4250 },
  { user_id: 'u2', name: 'Priya', balance_cents: 3100 },
];

function makePreview(overrides: Partial<SettlementPreview> = {}): SettlementPreview {
  return {
    bands: {
      base: {
        hours: 2,
        court_cents: 6000,
        shuttle_units: 10,
        shuttle_cents: 4167,
        total_cents: 10167,
        heads: 2,
      },
      extra: null,
    },
    totals: {
      court_cents: 6000,
      shuttle_cents: 4167,
      shuttle_units: 10,
      charged_cents: 10167,
      surplus_cents: 0,
    },
    lines: [
      { user_id: 'u1', name: 'Karthik', in_base: true, in_extra: false, comped: false, amount_cents: 5084 },
      { user_id: 'u2', name: 'Priya', in_base: true, in_extra: false, comped: false, amount_cents: 5083 },
    ],
    stock_after: { units: 14, amount_cents: 5833 },
    ...overrides,
  };
}

function shortfallError() {
  return {
    response: {
      data: {
        code: 'shuttle_stock_short',
        message: 'This session uses 10 shuttles but stock holds 3.',
        details: { required_units: 10, available_units: 3 },
      },
    },
  };
}

function lastCall<T extends unknown[]>(calls: T[]): T {
  return calls[calls.length - 1];
}

/** Names appear in both the participant list and the breakdown, so scope to one. */
function participants() {
  return within(screen.getByRole('list', { name: 'Participants' }));
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/admin/sessions/s1/settle']}>
      <Routes>
        <Route path="/admin/sessions/:id/settle" element={<AdminSettlement />} />
      </Routes>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listBalances).mockResolvedValue(members);
  vi.mocked(api.previewSettlement).mockResolvedValue(makePreview());
});

describe('AdminSettlement', () => {
  it('seeds the form from who said they were coming', async () => {
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    expect(participants().getByText('Priya')).toBeInTheDocument();
  });

  it('shows each person’s share and the total it will charge', async () => {
    renderPage();

    await waitFor(() => expect(participants().getByText('$50.84')).toBeInTheDocument());
    expect(participants().getByText('$50.83')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Settle — charge \$101\.67/ })
    ).toBeInTheDocument();
  });

  // The odd cent is the point: 10167 across two is 5083.5, and the total must
  // still come to exactly 10167.
  it('shows shares that sum to the total', async () => {
    renderPage();

    await waitFor(() => expect(participants().getByText('$50.84')).toBeInTheDocument());
    const preview = vi.mocked(api.previewSettlement).mock.results[0].value as Promise<SettlementPreview>;
    const resolved = await preview;
    const sum = resolved.lines.reduce((total, line) => total + line.amount_cents, 0);
    expect(sum).toBe(resolved.totals.charged_cents);
  });

  it('re-costs when the extra hour is added', async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    vi.mocked(api.previewSettlement).mockClear();

    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');

    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalled());
    const [, input] = lastCall(vi.mocked(api.previewSettlement).mock.calls);
    expect(input?.extra_hours).toBe(1);
  });

  // The form re-costs on every change, so settling must send the same input the
  // displayed figures were derived from — otherwise the admin approves one
  // number and the club charges another.
  it('posts exactly what it last displayed', async () => {
    const user = userEvent.setup();
    vi.mocked(api.settleSession).mockResolvedValue(makePreview());
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());

    // Change something, so there is a preview to compare the settle against.
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');
    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalledTimes(2));

    await user.click(screen.getByRole('button', { name: /Settle — charge/ }));
    await waitFor(() => expect(api.settleSession).toHaveBeenCalled());

    const [sessionId, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    const [, previewed] = lastCall(vi.mocked(api.previewSettlement).mock.calls);
    expect(sessionId).toBe('s1');
    expect(settled).toEqual(previewed);
  });

  // Opening the form should cost one request, not two.
  it('costs the session once when opened', async () => {
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    expect(api.previewSettlement).toHaveBeenCalledTimes(1);
  });

  it('offers to record the missing shuttles rather than dead-ending', async () => {
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(makePreview())
      .mockRejectedValue(shortfallError());

    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');

    await waitFor(() =>
      expect(screen.getByText(/uses 10 shuttles but stock holds 3/)).toBeInTheDocument()
    );
    expect(screen.getByRole('button', { name: 'Record' })).toBeInTheDocument();
  });

  it('re-costs after the missing purchase is recorded', async () => {
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(makePreview())
      .mockRejectedValueOnce(shortfallError())
      .mockResolvedValue(makePreview());
    vi.mocked(api.recordShuttlePurchase).mockResolvedValue({} as never);

    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Record' })).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Record' }));

    await waitFor(() => expect(api.recordShuttlePurchase).toHaveBeenCalledWith(12, 5000, expect.any(String)));
    await waitFor(() =>
      expect(screen.queryByText(/uses 10 shuttles but stock holds 3/)).not.toBeInTheDocument()
    );
  });

  it('lets a participant be removed', async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(participants().getByText('Priya')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: 'Remove Priya' }));

    expect(participants().queryByText('Priya')).not.toBeInTheDocument();
  });
});

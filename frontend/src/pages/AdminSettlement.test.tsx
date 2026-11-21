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
    fingerprint: 'a'.repeat(64),
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

function previewChangedError() {
  return {
    response: {
      data: {
        code: 'preview_changed',
        message: 'The costs have changed since they were reviewed.',
      },
    },
  };
}

/** A preview whose total and fingerprint say which request it answers. */
function previewCharging(cents: number, fingerprint: string): SettlementPreview {
  const base = makePreview();
  return { ...base, fingerprint, totals: { ...base.totals, charged_cents: cents } };
}

/** A promise the test settles by hand, to control the order replies arrive in. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function settleButton() {
  return screen.getByRole('button', { name: /^(Settle|Working out the cost)/ });
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
  // Also drop any queued one-off replies a test did not use up, so one test's
  // staged previews cannot answer the next test's requests.
  vi.mocked(api.previewSettlement).mockReset();
  vi.mocked(api.settleSession).mockReset();
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
    expect(settled).toEqual({ ...previewed, expected_preview: 'a'.repeat(64) });
  });

  // The server costs the session again when settling, and posts only if it
  // reaches the figures named by this fingerprint.
  it('sends the fingerprint of the preview on screen', async () => {
    const user = userEvent.setup();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockResolvedValueOnce(previewCharging(13167, 'b'.repeat(64)));
    vi.mocked(api.settleSession).mockResolvedValue(makePreview());
    renderPage();

    await waitFor(() => expect(participants().getByText('Karthik')).toBeInTheDocument());
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');

    await user.click(await screen.findByRole('button', { name: /Settle — charge \$131\.67/ }));

    await waitFor(() => expect(api.settleSession).toHaveBeenCalledTimes(1));
    const [, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    expect(settled?.expected_preview).toBe('b'.repeat(64));
    expect(settled?.extra_hours).toBe(1);
  });

  it('settles the opening preview with its fingerprint when nothing is changed', async () => {
    const user = userEvent.setup();
    vi.mocked(api.settleSession).mockResolvedValue(makePreview());
    renderPage();

    await user.click(await screen.findByRole('button', { name: /Settle — charge \$101\.67/ }));

    await waitFor(() => expect(api.settleSession).toHaveBeenCalledTimes(1));
    const [, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    expect(settled).toEqual({
      extra_hours: 0,
      lines: [
        { user_id: 'u1', guest_name: undefined, in_base: true, in_extra: false, comped: false },
        { user_id: 'u2', guest_name: undefined, in_base: true, in_extra: false, comped: false },
      ],
      expected_preview: 'a'.repeat(64),
    });
  });

  it('cannot settle while a preview is in flight, and shows no old total', async () => {
    const user = userEvent.setup();
    const pending = deferred<SettlementPreview>();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockReturnValueOnce(pending.promise);
    renderPage();

    await screen.findByRole('button', { name: /Settle — charge \$101\.67/ });
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');

    // The form now says three hours; the only total the page has is for two.
    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalledTimes(2));
    expect(settleButton()).toBeDisabled();
    expect(settleButton()).toHaveTextContent('Working out the cost…');
    expect(screen.queryByText(/\$101\.67/)).not.toBeInTheDocument();
    expect(participants().queryByText('$50.84')).not.toBeInTheDocument();

    await user.click(settleButton());
    expect(api.settleSession).not.toHaveBeenCalled();

    pending.resolve(previewCharging(13167, 'b'.repeat(64)));
    const ready = await screen.findByRole('button', { name: /Settle — charge \$131\.67/ });
    expect(ready).toBeEnabled();
  });

  // Two quick changes, answered in the wrong order. The slow answer to the
  // first change must not replace the answer to the second.
  it('ignores a reply to a preview that has been superseded', async () => {
    const user = userEvent.setup();
    const first = deferred<SettlementPreview>();
    const second = deferred<SettlementPreview>();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    vi.mocked(api.settleSession).mockResolvedValue(makePreview());
    renderPage();

    await screen.findByRole('button', { name: /Settle — charge \$101\.67/ });
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '2');
    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalledTimes(3));

    second.resolve(previewCharging(16167, 'c'.repeat(64)));
    await screen.findByRole('button', { name: /Settle — charge \$161\.67/ });

    // The answer for one extra hour arrives late.
    first.resolve(previewCharging(13167, 'b'.repeat(64)));
    await first.promise;
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /Settle — charge \$161\.67/ })).toBeEnabled()
    );
    expect(screen.queryByText(/\$131\.67/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Settle — charge \$161\.67/ }));
    await waitFor(() => expect(api.settleSession).toHaveBeenCalledTimes(1));
    const [, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    expect(settled?.extra_hours).toBe(2);
    expect(settled?.expected_preview).toBe('c'.repeat(64));
  });

  it('keeps settle disabled when the superseded reply arrives first', async () => {
    const user = userEvent.setup();
    const first = deferred<SettlementPreview>();
    const second = deferred<SettlementPreview>();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    renderPage();

    await screen.findByRole('button', { name: /Settle — charge \$101\.67/ });
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '2');
    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalledTimes(3));

    first.resolve(previewCharging(13167, 'b'.repeat(64)));
    await first.promise;
    await Promise.resolve();

    expect(settleButton()).toBeDisabled();
    expect(screen.queryByText(/\$131\.67/)).not.toBeInTheDocument();

    second.resolve(previewCharging(16167, 'c'.repeat(64)));
    await screen.findByRole('button', { name: /Settle — charge \$161\.67/ });
  });

  it('ignores a superseded preview that fails', async () => {
    const user = userEvent.setup();
    const first = deferred<SettlementPreview>();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockReturnValueOnce(first.promise)
      .mockResolvedValueOnce(previewCharging(16167, 'c'.repeat(64)));
    renderPage();

    await screen.findByRole('button', { name: /Settle — charge \$101\.67/ });
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '1');
    await user.selectOptions(screen.getByLabelText('Did you play on?'), '2');
    await screen.findByRole('button', { name: /Settle — charge \$161\.67/ });

    first.reject(shortfallError());
    await first.promise.catch(() => undefined);
    await Promise.resolve();

    expect(screen.queryByText(/uses 10 shuttles but stock holds 3/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Settle — charge \$161\.67/ })).toBeEnabled();
  });

  // The server refused because stock, rates or attendance moved after the
  // figures were shown. Nothing was charged, and the admin must see the new
  // figures before anything is.
  it('re-costs and asks again when the server says the costs changed', async () => {
    const user = userEvent.setup();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockResolvedValueOnce(previewCharging(12250, 'd'.repeat(64)));
    vi.mocked(api.settleSession)
      .mockRejectedValueOnce(previewChangedError())
      .mockResolvedValueOnce(makePreview());
    renderPage();

    await user.click(await screen.findByRole('button', { name: /Settle — charge \$101\.67/ }));

    const again = await screen.findByRole('button', { name: /Settle — charge \$122\.50/ });
    expect(screen.getByText(/costs changed after you reviewed them/)).toBeInTheDocument();
    expect(api.settleSession).toHaveBeenCalledTimes(1);

    await user.click(again);
    await waitFor(() => expect(api.settleSession).toHaveBeenCalledTimes(2));
    const [, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    expect(settled?.expected_preview).toBe('d'.repeat(64));
  });

  it('settles a guest through the same reviewed preview', async () => {
    const user = userEvent.setup();
    vi.mocked(api.previewSettlement)
      .mockResolvedValueOnce(previewCharging(10167, 'a'.repeat(64)))
      .mockResolvedValueOnce(previewCharging(10167, 'e'.repeat(64)));
    vi.mocked(api.settleSession).mockResolvedValue(makePreview());
    renderPage();

    await screen.findByRole('button', { name: /Settle — charge \$101\.67/ });
    await user.type(screen.getByLabelText('Add a guest'), 'Sam');
    await user.selectOptions(screen.getByLabelText('Charged to'), 'u1');
    const addButtons = screen.getAllByRole('button', { name: '' });
    await user.click(addButtons[addButtons.length - 1]);

    await waitFor(() => expect(api.previewSettlement).toHaveBeenCalledTimes(2));
    const [, previewed] = lastCall(vi.mocked(api.previewSettlement).mock.calls);
    expect(previewed?.lines).toHaveLength(3);
    expect(previewed?.lines?.[2]).toMatchObject({ user_id: 'u1', guest_name: 'Sam', in_base: true });

    await user.click(await screen.findByRole('button', { name: /Settle — charge/ }));
    await waitFor(() => expect(api.settleSession).toHaveBeenCalledTimes(1));
    const [, settled] = lastCall(vi.mocked(api.settleSession).mock.calls);
    expect(settled).toEqual({ ...previewed, expected_preview: 'e'.repeat(64) });
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

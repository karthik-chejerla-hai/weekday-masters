import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuth } from '../../context/useAuth';
import { api } from '../../services/api';
import type { PersonalSpend } from '../../types';
import SpendChip from './SpendChip';

vi.mock('../../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../../services/api', () => ({ api: { getMySpend: vi.fn() } }));

const spend: PersonalSpend = {
  year: 2026, as_of: '2026-10-05', recorded_from: '2025-12-01',
  ytd_cents: 12345, all_time_cents: 22345, months: [],
};
function auth(id = 'u1', isApproved = true) {
  vi.mocked(useAuth).mockReturnValue({ user: { id }, isApproved } as ReturnType<typeof useAuth>);
}
function Location() {
  const location = useLocation();
  return <p>{location.pathname}{location.search}</p>;
}
function View() {
  return <MemoryRouter><SpendChip /><Location /></MemoryRouter>;
}
beforeEach(() => {
  vi.resetAllMocks();
  auth();
  vi.mocked(api.getMySpend).mockResolvedValue(spend);
});

describe('Spend chip', () => {
  it('shows personal YTD spend and links to Analytics', async () => {
    render(<View />);
    const chip = await screen.findByRole('link', { name: /Your year-to-date spend: \$123.45/ });
    expect(chip).toHaveTextContent('2026 spend');
    expect(chip).not.toHaveTextContent('$223.45');
    await userEvent.click(chip);
    expect(screen.getByText('/money?tab=analytics')).toBeInTheDocument();
  });

  it('refreshes after an expense or reversal changes balances', async () => {
    render(<View />);
    await screen.findByText('$123.45');
    vi.mocked(api.getMySpend).mockResolvedValue({ ...spend, ytd_cents: 6789 });
    act(() => window.dispatchEvent(new Event('rally:balances-changed')));
    expect(await screen.findByText('$67.89')).toBeInTheDocument();
    expect(screen.queryByText('$123.45')).not.toBeInTheDocument();
  });

  it('does not show a failed request as zero spend', async () => {
    vi.mocked(api.getMySpend).mockRejectedValue(new Error('network'));
    render(<View />);
    expect(await screen.findByText('View details')).toBeInTheDocument();
    expect(screen.queryByText('$0.00')).not.toBeInTheDocument();
  });

  it('does not request or reveal spend for an unapproved member', async () => {
    auth('u1', false);
    render(<View />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(api.getMySpend).not.toHaveBeenCalled();
  });

  it('discards responses for a previous member', async () => {
    let resolveOld!: (data: PersonalSpend) => void;
    vi.mocked(api.getMySpend).mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }));
    const view = render(<View />);
    await waitFor(() => expect(api.getMySpend).toHaveBeenCalledOnce());
    auth('u2');
    vi.mocked(api.getMySpend).mockResolvedValue({ ...spend, ytd_cents: 999 });
    view.rerender(<View />);
    expect(await screen.findByText('$9.99')).toBeInTheDocument();
    await act(async () => resolveOld(spend));
    expect(screen.queryByText('$123.45')).not.toBeInTheDocument();
    expect(screen.getByText('$9.99')).toBeInTheDocument();
  });
});

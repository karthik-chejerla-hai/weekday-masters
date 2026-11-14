import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api } from '../../services/api';
import BalancesList from './BalancesList';
import type { BalanceNudgeResult, PlayerBalance } from '../../types';
vi.mock('../../services/api', () => ({ api: { nudgeBalance: vi.fn() } }));
const balances = [
 { user_id: 'low', name: 'Low', balance_cents: 1999 },
 { user_id: 'threshold', name: 'Threshold', balance_cents: 2000 },
 { user_id: 'self', name: 'Self', balance_cents: -100 },
] as PlayerBalance[];
const result = { notification_id: 'n1', balance_cents: 1999, push_sent: false, delivery_state: 'pending', next_allowed_at: '' } as BalanceNudgeResult;
beforeEach(() => vi.resetAllMocks());
it('uses the fixed threshold, excludes self, and reuses request identity after a timeout', async () => {
 vi.mocked(api.nudgeBalance).mockRejectedValueOnce(new Error('timeout')).mockResolvedValueOnce(result);
 const user = userEvent.setup(); render(<BalancesList balances={balances} lowThresholdCents={2000} currentUserId="self" canNudge />);
 const button = screen.getByRole('button', { name: /nudge/i });
 expect(screen.getAllByRole('button', { name: /nudge/i })).toHaveLength(1);
 await user.click(button); expect(await screen.findByRole('alert')).toHaveTextContent('Could not send');
 const firstRequest = vi.mocked(api.nudgeBalance).mock.calls[0][1]; expect(firstRequest).toMatch(/^[0-9a-f-]{36}$/);
 await user.click(button); expect(await screen.findByRole('status')).toHaveTextContent('queued');
 expect(api.nudgeBalance).toHaveBeenLastCalledWith('low', firstRequest);
});
it('reports suppression without claiming delivery', async () => {
 vi.mocked(api.nudgeBalance).mockResolvedValue({ ...result, delivery_state: 'suppressed' });
 const user = userEvent.setup(); render(<BalancesList balances={balances} lowThresholdCents={2000} currentUserId="self" canNudge />);
 await user.click(screen.getByRole('button', { name: /nudge/i }));
 expect(await screen.findByRole('status')).toHaveTextContent('recorded in Rally');
 expect(screen.queryByText(/sent to/i)).not.toBeInTheDocument();
});

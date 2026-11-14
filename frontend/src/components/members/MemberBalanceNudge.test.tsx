import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MemberBalanceNudge from './MemberBalanceNudge';
import { api } from '../../services/api';
import type { PlayerBalance } from '../../types';
vi.mock('../../services/api', () => ({ api: { nudgeBalance: vi.fn() } }));
const balance = { user_id: 'member', name: 'Alex', balance_cents: -100 } as PlayerBalance;
beforeEach(() => vi.resetAllMocks());
it('retains a failed request ID for member management retries', async () => {
 vi.mocked(api.nudgeBalance).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ notification_id: 'n1', balance_cents: -100, push_sent: false, delivery_state: 'suppressed', next_allowed_at: '' });
 const user = userEvent.setup(); render(<MemberBalanceNudge balance={balance} />);
 await user.click(screen.getByRole('button', { name: 'Nudge to top up' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('Try again');
 const request = vi.mocked(api.nudgeBalance).mock.calls[0][1];
 await user.click(screen.getByRole('button', { name: 'Nudge to top up' }));
 expect(await screen.findByRole('status')).toHaveTextContent('recorded');
 expect(api.nudgeBalance).toHaveBeenLastCalledWith('member', request);
 expect(screen.getByRole('button', { name: 'Reminder recorded' })).toBeDisabled();
});
it('offers no nudge at exactly $20', () => {
 render(<MemberBalanceNudge balance={{ ...balance, balance_cents: 2000 }} />);
 expect(screen.queryByRole('button')).not.toBeInTheDocument();
});

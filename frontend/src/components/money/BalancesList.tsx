import { useState } from 'react';
import { Bell, Loader2 } from 'lucide-react';
import Avatar from '../ui/Avatar';
import BalanceChip from './BalanceChip';
import { balanceState, formatCents } from './format';
import { api } from '../../services/api';
import type { PlayerBalance } from '../../types';

interface BalancesListProps {
  balances: PlayerBalance[];
  lowThresholdCents: number;
  currentUserId?: string;
  canNudge?: boolean;
}

type NudgeStatus = {
  state: 'sending' | 'sent' | 'error';
  message?: string;
};

function apiMessage(error: unknown) {
  return (error as { response?: { data?: { message?: string } } })?.response?.data?.message
    || 'Could not send the nudge. Please try again.';
}

/**
 * Everyone's balance, visible to everyone.
 *
 * The club already worked this way in Splitwise and is comfortable with it, and
 * it saves the admin from being the only person who can answer "am I square?".
 */
export default function BalancesList({ balances, lowThresholdCents, currentUserId, canNudge = false }: BalancesListProps) {
  const [nudges, setNudges] = useState<Record<string, NudgeStatus>>({});

  if (balances.length === 0) {
    return <p className="text-sm text-slate-500 py-8 text-center">No members yet.</p>;
  }

  // Whoever is furthest behind is who the admin needs to see first.
  const ordered = [...balances].sort((a, b) => a.balance_cents - b.balance_cents);
  const owing = ordered.filter((b) => b.balance_cents < 0);
  const clubTotal = balances.reduce((sum, b) => sum + b.balance_cents, 0);

  async function nudge(balance: PlayerBalance) {
    setNudges((current) => ({ ...current, [balance.user_id]: { state: 'sending' } }));
    try {
      const result = await api.nudgeBalance(balance.user_id);
      setNudges((current) => ({
        ...current,
        [balance.user_id]: {
          state: 'sent',
          message: result.push_sent
            ? `Push nudge queued for ${balance.name}.`
            : `${balance.name} will see the nudge in Rally; push is unavailable or disabled.`,
        },
      }));
    } catch (error) {
      setNudges((current) => ({
        ...current,
        [balance.user_id]: { state: 'error', message: apiMessage(error) },
      }));
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-baseline justify-between px-1">
        <span className="text-sm text-slate-600">
          {owing.length === 0
            ? 'Everyone is in credit'
            : `${owing.length} ${owing.length === 1 ? 'member owes' : 'members owe'} the club`}
        </span>
        <span className="text-sm text-slate-500 tabular-nums">
          {formatCents(clubTotal)} held
        </span>
      </div>

      <ul className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white">
        {ordered.map((balance) => {
          const nudgeStatus = nudges[balance.user_id];
          const showNudge = canNudge
            && balance.user_id !== currentUserId
            && balance.balance_cents < lowThresholdCents;
          return (
            <li key={balance.user_id} className="flex items-start gap-3 px-4 py-3">
              <Avatar src={balance.profile_picture} name={balance.name} size="sm" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-slate-800">
                  {balance.name}
                  {balance.user_id === currentUserId && (
                    <span className="ml-2 text-xs font-normal text-slate-400">you</span>
                  )}
                </p>
                {nudgeStatus?.message && (
                  <p
                    role={nudgeStatus.state === 'error' ? 'alert' : 'status'}
                    className={`mt-1 text-xs ${nudgeStatus.state === 'error' ? 'text-red-600' : 'text-slate-500'}`}
                  >
                    {nudgeStatus.message}
                  </p>
                )}
              </div>
              <div className="flex shrink-0 flex-col items-end gap-2">
                <BalanceChip
                  cents={balance.balance_cents}
                  state={balanceState(balance.balance_cents, lowThresholdCents)}
                  compact
                />
                {showNudge && (
                  <button
                    type="button"
                    className="inline-flex min-h-9 items-center gap-1 rounded-lg border border-amber-200 bg-amber-50 px-2.5 text-xs font-semibold text-amber-800 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={nudgeStatus?.state === 'sending' || nudgeStatus?.state === 'sent'}
                    onClick={() => void nudge(balance)}
                    aria-label={`Nudge ${balance.name} to top up`}
                  >
                    {nudgeStatus?.state === 'sending'
                      ? <><Loader2 className="h-3.5 w-3.5 animate-spin" /> Sending…</>
                      : nudgeStatus?.state === 'sent'
                        ? 'Nudged'
                        : <><Bell className="h-3.5 w-3.5" /> Nudge</>}
                  </button>
                )}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

import Avatar from '../ui/Avatar';
import BalanceChip from './BalanceChip';
import { balanceState, formatCents } from './format';
import type { PlayerBalance } from '../../types';

interface BalancesListProps {
  balances: PlayerBalance[];
  lowThresholdCents: number;
  currentUserId?: string;
}

/**
 * Everyone's balance, visible to everyone.
 *
 * The club already worked this way in Splitwise and is comfortable with it, and
 * it saves the admin from being the only person who can answer "am I square?".
 */
export default function BalancesList({ balances, lowThresholdCents, currentUserId }: BalancesListProps) {
  if (balances.length === 0) {
    return <p className="text-sm text-slate-500 py-8 text-center">No members yet.</p>;
  }

  // Whoever is furthest behind is who the admin needs to see first.
  const ordered = [...balances].sort((a, b) => a.balance_cents - b.balance_cents);
  const owing = ordered.filter((b) => b.balance_cents < 0);
  const clubTotal = balances.reduce((sum, b) => sum + b.balance_cents, 0);

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
        {ordered.map((balance) => (
          <li key={balance.user_id} className="flex items-center gap-3 px-4 py-3">
            <Avatar src={balance.profile_picture} name={balance.name} size="sm" />
            <span className="flex-1 truncate text-sm font-medium text-slate-800">
              {balance.name}
              {balance.user_id === currentUserId && (
                <span className="ml-2 text-xs font-normal text-slate-400">you</span>
              )}
            </span>
            <BalanceChip
              cents={balance.balance_cents}
              state={balanceState(balance.balance_cents, lowThresholdCents)}
              compact
            />
          </li>
        ))}
      </ul>
    </div>
  );
}

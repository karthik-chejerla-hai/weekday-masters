import { ArrowLeftRight, ArrowUpRight, Clapperboard, Landmark, PiggyBank, ReceiptText, RotateCcw, Split, Utensils } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { formatCents } from './format';
import ShuttleIcon from './ShuttleIcon';
import LedgerGameRow from './LedgerGameRow';
import type { LedgerActivityView } from '../../types';

const CATEGORIES = {
  topup: { label: 'Top-up', Icon: PiggyBank, style: 'bg-emerald-50 text-emerald-700' },
  session: { label: 'Session', Icon: ShuttleIcon, style: 'bg-primary-50 text-primary-700' },
  food: { label: 'Food and drink', Icon: Utensils, style: 'bg-orange-50 text-orange-700' },
  entertainment: { label: 'Entertainment', Icon: Clapperboard, style: 'bg-violet-50 text-violet-700' },
  shuttles: { label: 'Shuttles', Icon: ShuttleIcon, style: 'bg-primary-50 text-primary-700' },
  court_credit: { label: 'Court credit', Icon: Landmark, style: 'bg-sky-50 text-sky-700' },
  transfer: { label: 'Member transfer', Icon: ArrowLeftRight, style: 'bg-slate-100 text-slate-600' },
  withdrawal: { label: 'Paid out', Icon: ArrowUpRight, style: 'bg-slate-100 text-slate-600' },
  reversal: { label: 'Reversal', Icon: RotateCcw, style: 'bg-slate-100 text-slate-600' },
  opening_balance: { label: 'Opening balance', Icon: ReceiptText, style: 'bg-slate-100 text-slate-600' },
  other: { label: 'Other transaction', Icon: ReceiptText, style: 'bg-slate-100 text-slate-600' },
};

export default function LedgerList({ entries, showMember = false, userId }: { entries: LedgerActivityView[]; showMember?: boolean; userId?: string }) {
  return (
    <ul aria-label="Transactions" className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white">
      {entries.map((activity) => {
        if (activity.game) return <LedgerGameRow key={activity.id} id={activity.id} game={activity.game} occurredAt={activity.occurred_at} userId={userId} mineOnly={!showMember} />;
        const entry = activity.entry;
        if (!entry) return null;
        const isCredit = entry.amount_cents >= 0;
        const { label, Icon, style } = CATEGORIES[entry.category] ?? CATEGORIES.other;
        return (
          <li key={entry.id} className="flex items-start gap-3 p-4">
            <span className={`mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${style}`} role="img" aria-label={label}>
              <Icon className="h-5 w-5" aria-hidden="true" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-start justify-between gap-3">
                <p className="break-words text-sm font-semibold text-slate-900">{entry.description.trim() || label}</p>
                <span className={`shrink-0 text-sm font-semibold tabular-nums ${isCredit ? 'text-primary-700' : 'text-slate-700'}`}>
                  {isCredit ? '+' : ''}{formatCents(entry.amount_cents)}
                </span>
              </div>
              <div className="mt-1 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-xs text-slate-500">
                <span>{formatInTimeZone(entry.occurred_at, 'Australia/Sydney', 'd MMM yyyy')} · {label}</span>
                <span className="tabular-nums">Balance {formatCents(entry.balance_after_cents)}</span>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
                {showMember && <span className="font-medium text-slate-700">{entry.member_name}{entry.inactive ? ' · Inactive' : ''}</span>}
                {entry.source === 'splitwise' && <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 text-emerald-800" title="Imported from Splitwise">
                  <Split className="h-3 w-3" aria-hidden="true" />Source: Splitwise
                </span>}
                {entry.reversed && <span className="rounded-md bg-slate-100 px-2 py-0.5 text-slate-600">Reversed</span>}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

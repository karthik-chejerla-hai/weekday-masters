import { format, parseISO } from 'date-fns';
import { formatCents } from './format';
import type { LedgerEntryView, TransactionKind } from '../../types';

const LABELS: Record<TransactionKind, string> = {
  player_topup: 'Top-up',
  withdrawal: 'Paid out',
  court_credit_purchase: 'Court credit',
  shuttle_purchase: 'Shuttles',
  session_settlement: 'Session',
  opening_balance: 'Opening balance',
  reversal: 'Reversal',
};

interface LedgerListProps {
  entries: LedgerEntryView[];
}

/**
 * A member's own history, newest first.
 *
 * Every row carries the balance it produced, so the arithmetic can be followed
 * down the page rather than re-added by hand. That is the thing Splitwise never
 * made easy and the reason people stopped trusting it.
 */
export default function LedgerList({ entries }: LedgerListProps) {
  if (entries.length === 0) {
    return (
      <p className="text-sm text-slate-500 py-8 text-center">
        Nothing here yet. Top-ups and session charges will appear as they happen.
      </p>
    );
  }

  return (
    <ul className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white">
      {entries.map((entry) => {
        const isCredit = entry.amount_cents >= 0;
        return (
          <li key={entry.id} className="px-4 py-3">
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-sm font-medium text-slate-800">
                {LABELS[entry.kind] ?? entry.kind}
                {entry.reversed && (
                  <span className="ml-2 rounded bg-slate-100 px-1.5 py-0.5 text-xs font-normal text-slate-500">
                    reversed
                  </span>
                )}
              </span>
              <span
                className={`text-sm font-semibold tabular-nums ${
                  isCredit ? 'text-primary-700' : 'text-slate-700'
                }`}
              >
                {isCredit ? '+' : ''}
                {formatCents(entry.amount_cents)}
              </span>
            </div>

            <div className="mt-0.5 flex items-baseline justify-between gap-3">
              <span className="truncate text-xs text-slate-500">
                {format(parseISO(entry.occurred_at), 'd MMM yyyy')}
                {entry.description ? ` · ${entry.description}` : ''}
              </span>
              <span className="text-xs text-slate-400 tabular-nums">
                {formatCents(entry.balance_after_cents)}
              </span>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

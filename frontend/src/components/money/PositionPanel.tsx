import { AlertTriangle, CheckCircle2 } from 'lucide-react';
import { formatCents } from './format';
import type { ClubPosition } from '../../types';

interface PositionPanelProps {
  position: ClubPosition;
}

/**
 * Whether the club is square with its players.
 *
 * The point of showing three asset lines rather than one total is that only the
 * first is cash. The other two are money already spent on things the club will
 * consume — credit at the venue, and shuttles in a bag — and a single number
 * cannot tell you whether you are covered.
 */
export default function PositionPanel({ position }: PositionPanelProps) {
  const { assets, liabilities, surplus_cents, balanced, warnings } = position;

  return (
    <div className="space-y-4">
      {warnings.map((warning) => (
        <div
          key={warning.code}
          className="flex items-start gap-2 rounded-lg border border-secondary-300 bg-secondary-50 p-3"
        >
          <AlertTriangle className="w-5 h-5 text-secondary-600 flex-shrink-0 mt-0.5" />
          <p className="text-sm text-secondary-900">{warning.message}</p>
        </div>
      ))}

      <div className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
        <Row label="In the bank" value={assets.bank_cents} />
        <Row label="Credit at the venue" value={assets.court_credit_cents} />
        <Row
          label="Shuttles in the bag"
          value={assets.shuttle_stock_cents}
          note={`${assets.shuttle_stock_units} left`}
        />
        <Row label="What the club holds" value={assets.total_cents} strong />
      </div>

      <div className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
        <Row label="Members have prepaid" value={liabilities.player_balances_cents} />
        <Row
          label="Club surplus"
          value={surplus_cents}
          note={surplus_cents < 0 ? 'given away' : undefined}
        />
      </div>

      <div
        className={`flex items-center gap-2 rounded-lg border p-3 text-sm ${
          balanced
            ? 'border-primary-200 bg-primary-50 text-primary-800'
            : 'border-red-200 bg-red-50 text-red-800'
        }`}
      >
        {balanced ? (
          <>
            <CheckCircle2 className="w-5 h-5 flex-shrink-0" />
            <span>The books balance. What the club holds matches what members have prepaid.</span>
          </>
        ) : (
          <>
            <AlertTriangle className="w-5 h-5 flex-shrink-0" />
            <span>
              The books do not balance. Something wrote to the ledger without going through
              the usual path — check the integrity report.
            </span>
          </>
        )}
      </div>
    </div>
  );
}

function Row({
  label,
  value,
  note,
  strong,
}: {
  label: string;
  value: number;
  note?: string;
  strong?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between px-4 py-3">
      <span className={`text-sm ${strong ? 'font-semibold text-slate-900' : 'text-slate-700'}`}>
        {label}
        {note && <span className="ml-2 text-xs text-slate-400">{note}</span>}
      </span>
      <span
        className={`text-sm tabular-nums ${
          strong ? 'font-semibold text-slate-900' : 'text-slate-700'
        }`}
      >
        {formatCents(value)}
      </span>
    </div>
  );
}

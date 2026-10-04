import { format, parseISO } from 'date-fns';
import type { ImportedSession } from '../../types';
import { formatCents } from '../money/format';

export default function ImportedBreakdown({ session }: { session: ImportedSession }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-500">
        {session.date_basis === 'recorded' ? 'Recorded' : 'Played'} {format(parseISO(session.played_date), 'EEEE d MMMM yyyy')}
        {' · Imported from Splitwise'}
      </p>
      <div className="rounded-xl border border-slate-200 bg-white p-4 flex justify-between font-semibold">
        <span>Total charged</span><span>{formatCents(session.total_cents)}</span>
      </div>
      <ul className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
        {session.lines.map((line) => (
          <li key={line.user_id ?? line.name} className="px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <span>{line.name}{line.inactive && <span className="ml-2 text-xs text-slate-500">Inactive</span>}</span>
              <span className="font-medium tabular-nums">{formatCents(line.charge_cents)}</span>
            </div>
            {line.paid_cents > 0 && (
              <p className="mt-1 text-xs text-slate-500">
                Paid {formatCents(line.paid_cents)} for the group. Balance change: {line.net_cents >= 0 ? '+' : ''}{formatCents(line.net_cents)}.
              </p>
            )}
          </li>
        ))}
      </ul>
      <div className="text-xs text-slate-500 space-y-1">
        {session.sources.map((source, index) => (
          <p key={index}>{source.description} · {formatCents(source.cost_cents)} · Recorded {format(parseISO(source.recorded_date), 'd MMM yyyy')}</p>
        ))}
      </div>
    </div>
  );
}

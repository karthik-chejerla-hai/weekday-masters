import { format, parseISO } from 'date-fns';
import { usePersonalSpend } from '../../hooks/usePersonalSpend';
import { formatCents } from './format';

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

export default function SpendAnalytics() {
  const { data, error, refresh } = usePersonalSpend();
  if (error) {
    return (
      <div role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
        Could not load your spend.
        <button className="ml-2 min-h-11 font-semibold underline" onClick={refresh}>Try again</button>
      </div>
    );
  }
  if (!data) return <div role="status" className="card p-8 text-center text-sm text-slate-500">Loading your spend…</div>;

  const maxMonth = Math.max(...data.months.map((month) => month.amount_cents), 1);
  return (
    <section className="space-y-6" aria-labelledby="spend-heading">
      <div>
        <h2 id="spend-heading" className="text-lg font-semibold text-slate-950">Your badminton spend</h2>
        <p className="mt-1 text-sm text-slate-500">Session charges to your account, including guests you cover.</p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-primary-200 bg-primary-50 p-5">
          <p className="text-sm font-semibold text-primary-800">Year to date · {data.year}</p>
          <p className="mt-2 text-3xl font-bold tracking-tight text-primary-950 tabular-nums">{formatCents(data.ytd_cents)}</p>
          <p className="mt-2 text-xs text-primary-800">1 Jan to {format(parseISO(data.as_of), 'd MMM yyyy')}</p>
        </div>
        <div className="card p-5">
          <p className="text-sm font-semibold text-slate-600">All time</p>
          <p className="mt-2 text-3xl font-bold tracking-tight text-slate-950 tabular-nums">{formatCents(data.all_time_cents)}</p>
          <p className="mt-2 text-xs text-slate-500">
            {data.recorded_from ? `Recorded sessions since ${format(parseISO(data.recorded_from), 'd MMM yyyy')}` : 'No recorded session charges yet'}
          </p>
        </div>
      </div>

      {data.recorded_from === null ? (
        <div className="card p-6 text-sm text-slate-600">Your spend will appear here after a session is settled. Deposits do not count as spend.</div>
      ) : (
        <div className="card p-5">
          <h3 className="font-semibold text-slate-900">Monthly spend · {data.year}</h3>
          <p className="mt-1 text-xs text-slate-500">By session date in Sydney. The current month is shown to date.</p>
          <dl className="mt-5 space-y-3">
            {data.months.map(({ month, amount_cents }) => (
              <div key={month} className="flex items-center gap-3 text-sm">
                <dt className="w-8 shrink-0 text-slate-500">{MONTHS[month - 1]}</dt>
                <div aria-hidden="true" className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-slate-100">
                  <div className="h-full rounded-full bg-primary-500" style={{ width: `${(amount_cents / maxMonth) * 100}%` }} />
                </div>
                <dd className="w-24 shrink-0 text-right font-medium text-slate-900 tabular-nums">{formatCents(amount_cents)}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}

      <p className="text-xs leading-relaxed text-slate-500">
        Includes recorded Rally and imported Splitwise session charges. Excludes deposits, withdrawals,
        opening balances and reversed charges. Unsettled sessions and spending outside this club are not included.
      </p>
    </section>
  );
}

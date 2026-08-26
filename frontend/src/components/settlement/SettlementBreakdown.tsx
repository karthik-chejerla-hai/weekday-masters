import { formatCents } from '../money/format';
import type { SettlementPreview, SettlementRates } from '../../types';

interface SettlementBreakdownProps {
  preview: SettlementPreview;
  rates?: SettlementRates;
  /** Historical settlements show the rates they were costed at, not today's. */
  showRates?: boolean;
}

/**
 * What a night cost and how it was shared.
 *
 * Readable by any member, deliberately: the people in a split are the people
 * best placed to notice if it is wrong.
 */
export default function SettlementBreakdown({ preview, rates, showRates }: SettlementBreakdownProps) {
  const { bands, totals, lines } = preview;
  const consumed = totals.court_cents + totals.shuttle_cents;

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
        <BandRow label="Standard hours" band={bands.base} />
        {bands.extra && <BandRow label="Extra hour" band={bands.extra} />}

        <div className="flex items-baseline justify-between px-4 py-3">
          <span className="text-sm font-semibold text-slate-900">Total</span>
          <span className="text-sm font-semibold text-slate-900 tabular-nums">
            {formatCents(consumed)}
          </span>
        </div>
      </div>

      {totals.surplus_cents !== 0 && (
        <p className="text-xs text-slate-500 px-1">
          {formatCents(Math.abs(totals.surplus_cents))} covered by the club.
        </p>
      )}

      <ul className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
        {lines.map((line, i) => (
          <li key={`${line.user_id}-${line.guest_name ?? ''}-${i}`} className="flex items-center gap-3 px-4 py-2.5">
            <span className="flex-1 truncate text-sm text-slate-800">
              {line.guest_name ? (
                <>
                  {line.guest_name}
                  <span className="ml-2 text-xs text-slate-400">guest of {line.name}</span>
                </>
              ) : (
                line.name
              )}
            </span>

            {!line.in_extra && bands.extra && (
              <span className="text-xs text-slate-400">left early</span>
            )}
            {line.comped && <span className="text-xs text-primary-600">comped</span>}

            <span className="text-sm font-medium text-slate-700 tabular-nums">
              {formatCents(line.amount_cents)}
            </span>
          </li>
        ))}
      </ul>

      {showRates && rates && (
        <p className="text-xs text-slate-400 px-1">
          Costed at {formatCents(rates.base_rate_cents)}/hour
          {rates.extra_hours > 0 && <> and {formatCents(rates.extra_rate_cents)}/hour for the extra hour</>},
          {' '}{rates.shuttles_per_hour} shuttles an hour. These are the rates in force at the time.
        </p>
      )}
    </div>
  );
}

function BandRow({ label, band }: { label: string; band: SettlementPreview['bands']['base'] }) {
  if (!band) return null;

  return (
    <div className="px-4 py-3">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-medium text-slate-800">{label}</span>
        <span className="text-sm text-slate-800 tabular-nums">{formatCents(band.total_cents)}</span>
      </div>
      <p className="mt-0.5 text-xs text-slate-500">
        {band.hours}h court {formatCents(band.court_cents)} · {band.shuttle_units} shuttles{' '}
        {formatCents(band.shuttle_cents)} · split {band.heads} ways
      </p>
    </div>
  );
}

import { AlertTriangle, CheckCircle2, Landmark, Wallet } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import ShuttleIcon from './ShuttleIcon';
import { formatCents } from './format';
import type { ClubPosition } from '../../types';

interface PositionPanelProps {
  position: ClubPosition;
}

export default function PositionPanel({ position }: PositionPanelProps) {
  const { assets, liabilities, surplus_cents, balanced, warnings } = position;

  if (position.assets_pending) {
    return (
      <div className="rounded-lg border border-secondary-300 bg-secondary-50 p-4" role="status">
        <p className="font-semibold text-secondary-900">Club assets need review</p>
        <p className="mt-1 text-sm text-secondary-900">
          Member history is imported. Bank funds, court credit and shuttle stock still need to be confirmed.
        </p>
      </div>
    );
  }

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

      {!balanced && <div role="alert" className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
        <AlertTriangle className="h-5 w-5 shrink-0" aria-hidden="true" />
        <span>The books do not balance. Ask an admin to check the account records.</span>
      </div>}

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3 lg:gap-4" role="region" aria-label="Club asset cards">
        <AssetCard title="Unused Court Credit" value={formatCents(assets.court_credit_cents)} note="Available for next session" Icon={Wallet} />
        <AssetCard title="Shuttles available" value={formatCents(assets.shuttle_stock_cents)} note={`${assets.shuttle_stock_units} shuttles in the bag`} Icon={ShuttleIcon} />
        <AssetCard title="Bank Account balance" value={formatCents(assets.bank_cents)} note="Club funds" Icon={Landmark} />
      </div>

      <details className="rounded-xl border border-slate-200 bg-white">
        <summary className="min-h-11 cursor-pointer rounded-xl px-4 py-3 text-sm font-semibold text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-700">Reconciliation and audit details</summary>
        <div className="space-y-4 px-4 pb-4">
          <dl className="grid gap-3 text-sm sm:grid-cols-3">
            <div><dt className="font-semibold">Court credit</dt><dd>On: {assetDate(assets.court_credit_as_of)}</dd></div>
            <div><dt className="font-semibold">Shuttles</dt><dd>Audited on: {assetDate(assets.shuttle_audited_on)}</dd>
              {assets.shuttle_stock_as_of && assets.shuttle_stock_as_of !== assets.shuttle_audited_on && <dd>Stock updated: {assetDate(assets.shuttle_stock_as_of)}</dd>}
            </div>
            <div><dt className="font-semibold">Bank balance</dt><dd>On: {assetDate(assets.bank_as_of)}</dd></div>
          </dl>

          <div className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
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

          {balanced && <p className="flex items-center gap-2 rounded-lg border border-primary-200 bg-primary-50 p-3 text-sm text-primary-800">
            <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden="true" />
            The books balance. Assets match member balances plus club surplus.
          </p>}
        </div>
      </details>
    </div>
  );
}

function assetDate(value?: string | null) {
  return value ? format(parseISO(value), 'd MMM yyyy') : 'Not recorded';
}

function AssetCard({ title, value, note, Icon }: {
  title: string; value: string; note: string;
  Icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
}) {
  return (
    <article className="flex min-w-0 gap-3 rounded-2xl border border-primary-100 bg-white p-4 shadow-sm lg:flex-col lg:gap-0 lg:p-5">
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700 lg:mb-4 lg:h-10 lg:w-10"><Icon className="h-5 w-5" aria-hidden="true" /></span>
      <div className="flex min-w-0 flex-1 flex-col">
        <h2 className="text-sm font-semibold text-slate-700">{title}</h2>
        <p className="mt-1 text-2xl font-semibold tracking-tight text-slate-950 tabular-nums lg:mt-2 lg:text-3xl">{value}</p>
        <p className="mt-1 text-sm text-slate-500">{note}</p>
      </div>
    </article>
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

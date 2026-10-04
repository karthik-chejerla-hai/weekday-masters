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

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3 lg:gap-4" role="region" aria-label="Club asset cards">
        <AssetCard title="Unused Court Credit" value={formatCents(assets.court_credit_cents)} note="Available for next session" Icon={Wallet} date={`On: ${assetDate(assets.court_credit_as_of)}`} />
        <AssetCard title="Shuttles available" value={formatCents(assets.shuttle_stock_cents)} note={`${assets.shuttle_stock_units} shuttles in the bag`} Icon={ShuttleIcon} date={`Audited on: ${assetDate(assets.shuttle_audited_on)}`}>
          {assets.shuttle_stock_as_of && assets.shuttle_stock_as_of !== assets.shuttle_audited_on && (
            <p className="mt-1 text-xs text-slate-500">Stock updated: {assetDate(assets.shuttle_stock_as_of)}</p>
          )}
        </AssetCard>
        <AssetCard title="Bank Account balance" value={formatCents(assets.bank_cents)} note="Club funds" Icon={Landmark} date={`On: ${assetDate(assets.bank_as_of)}`} />
      </div>

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
            <span>The books balance. Assets match member balances plus club surplus.</span>
          </>
        ) : (
          <>
            <AlertTriangle className="w-5 h-5 flex-shrink-0" />
            <span>
              The books do not balance. Ask an admin to check the account records.
            </span>
          </>
        )}
      </div>
    </div>
  );
}

function assetDate(value?: string | null) {
  return value ? format(parseISO(value), 'd MMM yyyy') : 'Not recorded';
}

function AssetCard({ title, value, note, date, Icon, children }: {
  title: string; value: string; note: string; date: string;
  Icon: React.ComponentType<React.SVGProps<SVGSVGElement>>; children?: React.ReactNode;
}) {
  return (
    <article className="flex min-w-0 gap-3 rounded-2xl border border-primary-100 bg-white p-4 shadow-sm lg:flex-col lg:gap-0 lg:p-5">
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700 lg:mb-4 lg:h-10 lg:w-10"><Icon className="h-5 w-5" aria-hidden="true" /></span>
      <div className="flex min-w-0 flex-1 flex-col">
        <h2 className="text-sm font-semibold text-slate-700">{title}</h2>
        <p className="mt-1 text-2xl font-semibold tracking-tight text-slate-950 tabular-nums lg:mt-2 lg:text-3xl">{value}</p>
        <p className="mt-1 mb-3 text-sm text-slate-500 lg:mb-6">{note}</p>
        <div className="mt-auto border-t border-slate-100 pt-2 lg:pt-3">
          <p className="text-xs font-medium text-slate-500">{date}</p>
          {children}
        </div>
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

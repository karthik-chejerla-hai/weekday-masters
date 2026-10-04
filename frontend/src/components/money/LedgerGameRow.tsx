import { useState } from 'react';
import { ChevronDown, Split } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { formatInTimeZone } from 'date-fns-tz';
import type { LedgerGameView } from '../../types';
import { formatCents } from './format';
import ShuttleIcon from './ShuttleIcon';

export default function LedgerGameRow({ id, game, occurredAt, userId, mineOnly }: {
  id: string; game: LedgerGameView; occurredAt: string; userId?: string; mineOnly: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const ownShare = game.shares.find((share) => share.user_id === userId);
  const detailsId = `game-shares-${id}`;
  const recordedDate = formatInTimeZone(occurredAt, 'Australia/Sydney', 'yyyy-MM-dd');
  return (
    <li>
      <button type="button" className="flex w-full items-start gap-3 rounded-xl p-4 text-left transition-colors hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
        aria-expanded={expanded} aria-controls={detailsId} aria-label={`${expanded ? 'Hide' : 'Show'} shares for ${game.title}`}
        onClick={() => setExpanded((value) => !value)}>
        <span className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700" role="img" aria-label="Session">
          <ShuttleIcon className="h-5 w-5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-start justify-between gap-3">
            <span className="break-words text-sm font-semibold text-slate-900">{game.title}</span>
            <span className="shrink-0 text-right">
              <span className="block text-sm font-semibold tabular-nums text-slate-900">{formatCents(game.total_charged_cents)}</span>
              <span className="block text-xs text-slate-500">Total charged</span>
            </span>
          </span>
          <span className="mt-1 block text-xs text-slate-500">
            {game.date_basis === 'recorded' ? 'Recorded' : 'Played'} {format(parseISO(game.played_date), 'd MMM yyyy')}
            {game.date_basis !== 'recorded' && recordedDate !== game.played_date && ` · Recorded ${formatInTimeZone(occurredAt, 'Australia/Sydney', 'd MMM yyyy')}`}
          </span>
          <span className="mt-2 flex flex-wrap items-center gap-2 text-xs">
            <span className="font-medium text-slate-600">{game.shares.length} {game.shares.length === 1 ? 'member' : 'members'}</span>
            {mineOnly && ownShare && <span className="font-medium text-primary-700">Your share {formatCents(ownShare.charge_cents)}</span>}
            {game.source === 'splitwise' && <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 text-emerald-800">
              <Split className="h-3 w-3" aria-hidden="true" />Source: Splitwise
            </span>}
            {game.reversed && <span className="rounded-md bg-slate-100 px-2 py-0.5 text-slate-600">Reversed</span>}
          </span>
          <span className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-primary-700">
            {expanded ? 'Hide shares' : 'View shares'}<ChevronDown className={`h-4 w-4 transition-transform ${expanded ? 'rotate-180' : ''}`} aria-hidden="true" />
          </span>
        </span>
      </button>
      {expanded && (
        <div id={detailsId} role="region" aria-label={`${game.title} shares`} className="border-t border-slate-100 bg-slate-50/60 px-4 py-3 sm:pl-[4.5rem]">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead><tr className="border-b border-slate-200 text-xs text-slate-500">
                <th scope="col" className="py-2 pr-3 text-left font-medium">Member</th>
                <th scope="col" className="whitespace-nowrap px-3 py-2 text-right font-medium">Share</th>
                <th scope="col" className="whitespace-nowrap py-2 pl-3 text-right font-medium">Balance after</th>
              </tr></thead>
              <tbody className="divide-y divide-slate-100">
                {game.shares.map((share) => (
                  <tr key={share.id}>
                    <th scope="row" className="py-3 pr-3 text-left font-medium text-slate-700">
                      {share.member_name}{share.user_id === userId && <span className="ml-1 text-xs font-normal text-slate-500">(you)</span>}
                      {share.inactive && <span className="ml-2 text-xs font-normal text-slate-500">Inactive</span>}
                      {!!share.guest_names?.length && <span className="mt-1 block text-xs font-normal text-slate-500">Includes guest: {share.guest_names.join(', ')}</span>}
                      {share.paid_cents > 0 && <span className="mt-1 block text-xs font-normal text-slate-500">
                        Paid {formatCents(share.paid_cents)} for the group.
                        {' '}Balance change: {share.amount_cents >= 0 ? '+' : ''}{formatCents(share.amount_cents)}.
                      </span>}
                    </th>
                    <td className="whitespace-nowrap px-3 py-3 text-right tabular-nums text-slate-800">{formatCents(share.charge_cents)}</td>
                    <td className="whitespace-nowrap py-3 pl-3 text-right tabular-nums text-slate-500">{formatCents(share.balance_after_cents)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-2 text-xs text-slate-500">
            Balances are after this game was recorded.
            {game.source_count > 1 && ' Includes the regular and extra-hour charges.'}
          </p>
        </div>
      )}
    </li>
  );
}

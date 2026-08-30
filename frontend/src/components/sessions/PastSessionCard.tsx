import { Link } from 'react-router-dom';
import { format, parseISO } from 'date-fns';
import { formatCents } from '../money/format';
import type { PastSession } from '../../types';

interface PastSessionCardProps {
  session: PastSession;
  /** Admins get a way to act on a session nobody has costed yet. */
  isAdmin?: boolean;
}

/**
 * A finished session.
 *
 * Unsettled ones are shown rather than hidden — an uncosted session is exactly
 * the thing the admin needs reminding about.
 */
export default function PastSessionCard({ session, isAdmin }: PastSessionCardProps) {
  const played = session.ends_at ? parseISO(session.ends_at) : null;

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="font-semibold text-slate-900 truncate">{session.title}</h3>
          {played && (
            <p className="text-sm text-slate-500 mt-0.5">
              {format(played, 'EEE d MMM yyyy')}
            </p>
          )}
        </div>

        {session.settled ? (
          <span className="text-sm font-semibold text-slate-800 tabular-nums whitespace-nowrap">
            {formatCents(session.total_cents)}
          </span>
        ) : (
          <span className="rounded-full border border-secondary-300 bg-secondary-50 px-2 py-0.5 text-xs font-medium text-secondary-700 whitespace-nowrap">
            Not settled
          </span>
        )}
      </div>

      <div className="mt-3 flex items-center justify-between">
        {session.settled ? (
          <>
            <span className="text-sm text-slate-500">
              {session.player_count} {session.player_count === 1 ? 'player' : 'players'}
            </span>
            <Link
              to={`/sessions/${session.session_id}/settlement`}
              className="text-sm font-medium text-primary-600 hover:text-primary-700"
            >
              See the split
            </Link>
          </>
        ) : (
          <>
            <span className="text-sm text-slate-500">Nobody has costed this yet</span>
            {isAdmin && (
              <Link
                to={`/admin/sessions/${session.session_id}/settle`}
                className="text-sm font-medium text-primary-600 hover:text-primary-700"
              >
                Settle it
              </Link>
            )}
          </>
        )}
      </div>
    </div>
  );
}

import { Link } from 'react-router-dom';
import { Clock, MapPin, Timer } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { formatInTimeZone } from 'date-fns-tz';
import type { Session, TimeFormat, RSVPStatus } from '../../types';
import Badge from '../ui/Badge';
import PlayerRSVPChips from './PlayerRSVPChips';
import { formatSessionTimeRange } from '../../utils/session-display';

interface SessionCardProps {
  session: Session;
  venueName?: string;
  courtNumber?: number;
  timeFormat?: TimeFormat;
  featured?: boolean;
  recordScore?: boolean;
  currentUserId?: string;
}

const responseLabels: Record<RSVPStatus, string> = {
  in: 'Confirmed', maybe: 'Tentative', out: 'Not attending', waitlisted: 'Waitlisted',
};

export default function SessionCard({ session, venueName, courtNumber, timeFormat, featured = false, recordScore = false, currentUserId }: SessionCardProps) {
  const sessionDate = parseISO(session.session_date);
  const dateLabel = format(sessionDate, 'EEEE, d MMMM yyyy');
  // Recurring sessions use this date-only title. Preserve every custom title.
  const hasCustomTitle = session.title !== format(sessionDate, 'EEEE - dd MMM yyyy');
  const isDeadlinePassed = new Date() > new Date(session.rsvp_deadline);
  const isCancelled = session.status === 'cancelled';
  const isCurrentSession = recordScore && !isCancelled;
  const confirmedRsvps = session.rsvps?.filter(r => r.status === 'in') || [];
  const waitingCount = session.rsvps?.filter(r => r.status === 'waitlisted').length || 0;
  const myRsvp = session.rsvps?.find(r => r.user_id === currentUserId);
  const spotsLeft = session.max_players - confirmedRsvps.length;

  return (
    <article className={`overflow-hidden rounded-2xl border bg-white shadow-sm ${featured ? 'border-primary-200' : 'border-slate-200'}`}>
      <Link
        to={`/sessions/${session.id}`}
        aria-label={hasCustomTitle ? `${session.title}, ${dateLabel}` : dateLabel}
        className="block p-4 transition-colors hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-700 sm:p-5"
      >
        <h3 className="break-words font-semibold text-slate-950">
          {hasCustomTitle ? session.title : <time dateTime={format(sessionDate, 'yyyy-MM-dd')}>{dateLabel}</time>}
        </h3>
        {hasCustomTitle && <time dateTime={format(sessionDate, 'yyyy-MM-dd')} className="mt-1 block text-sm font-medium text-slate-700">{dateLabel}</time>}

        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-slate-600">
          <span className="inline-flex items-center gap-1.5 tabular-nums">
            <Clock className="h-4 w-4 shrink-0" aria-hidden="true" />
            {formatSessionTimeRange(session.start_time, session.end_time, timeFormat)}
          </span>
          <span className="inline-flex min-w-0 items-start gap-1.5">
            <MapPin className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="break-words"><span>{venueName?.trim() || 'Venue to be confirmed'}</span>{courtNumber !== undefined && courtNumber > 0 && <> · <span className="whitespace-nowrap">Court {courtNumber}</span></>}</span>
          </span>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
          <span className="font-medium text-slate-700">{isCurrentSession ? `${confirmedRsvps.length} confirmed ${confirmedRsvps.length === 1 ? 'player' : 'players'}` : `${confirmedRsvps.length} of ${session.max_players} confirmed`}</span>
          {!isCurrentSession && (isCancelled ? <Badge variant="danger">Cancelled</Badge> : <>
            {spotsLeft <= 0 ? <Badge variant="warning">Full{waitingCount > 0 ? ` · ${waitingCount} waiting` : ''}</Badge>
              : spotsLeft <= 2 && <Badge variant="warning">{spotsLeft} spots left</Badge>}
            {isDeadlinePassed ? <Badge variant="warning">RSVP Closed</Badge> : <Badge variant="success">RSVP open</Badge>}
          </>)}
        </div>
        {!isCurrentSession && <p className="mt-2 rounded-lg bg-slate-100 px-3 py-2 text-sm font-semibold text-slate-900">
          {isCancelled ? 'Your recorded RSVP: ' : 'Your RSVP: '}
          {myRsvp ? responseLabels[myRsvp.status] : 'Not responded'}
          {myRsvp?.status === 'waitlisted' && myRsvp.waitlist_position ? ` (#${myRsvp.waitlist_position})` : ''}
        </p>}

        <div className="mt-3">
          {isCurrentSession && confirmedRsvps.length === 0
            ? <p className="text-sm text-slate-500">No confirmed players yet</p>
            : <PlayerRSVPChips rsvps={isCurrentSession ? confirmedRsvps : session.rsvps ?? []} />}
        </div>
        {!isCurrentSession && !isDeadlinePassed && !isCancelled && (
          <p className="mt-3 flex items-start gap-1.5 text-xs text-amber-800">
            <Timer className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>RSVP by {formatInTimeZone(session.rsvp_deadline, 'Australia/Sydney', `EEE, d MMM 'at' ${timeFormat === '12h' ? 'h:mm a' : 'HH:mm'}`)}</span>
          </p>
        )}
        {!recordScore && <p className="mt-2 text-xs text-slate-500">Open session for details</p>}
      </Link>
      {recordScore && !isCancelled && <div className="flex flex-wrap items-center gap-3 border-t border-slate-100 p-4 sm:p-5">
        <Link to={`/sessions/${session.id}/games`} className="btn-primary min-h-11 flex-1 sm:flex-none">Record score</Link>
        <Link to={`/sessions/${session.id}`} className="inline-flex min-h-11 items-center px-2 text-sm font-semibold text-primary-700 underline underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-700 focus-visible:ring-offset-2">Session details</Link>
      </div>}
    </article>
  );
}

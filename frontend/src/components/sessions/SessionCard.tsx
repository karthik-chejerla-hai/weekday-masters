import { useNavigate } from 'react-router-dom';
import { Clock, MapPin, Timer } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import type { Session, TimeFormat } from '../../types';
import Badge from '../ui/Badge';
import PlayerRSVPChips from './PlayerRSVPChips';
import { formatSessionTimeRange } from '../../utils/session-display';

interface SessionCardProps {
  session: Session;
  venueName?: string;
  courtNumber?: number;
  timeFormat?: TimeFormat;
  featured?: boolean;
}

export default function SessionCard({ session, venueName, courtNumber, timeFormat, featured = false }: SessionCardProps) {
  const navigate = useNavigate();

  const sessionDate = parseISO(session.session_date);
  const isDeadlinePassed = new Date() > new Date(session.rsvp_deadline);

  const confirmedRsvps = session.rsvps?.filter(r => r.status === 'in') || [];
  const waitlistedRsvps = session.rsvps?.filter(r => r.status === 'waitlisted') || [];

  const confirmedCount = confirmedRsvps.length;
  const spotsLeft = session.max_players - confirmedCount;

  const handleCardClick = () => {
    navigate(`/sessions/${session.id}`);
  };

  return (
    <article className={`overflow-hidden rounded-2xl border bg-white shadow-sm ${featured ? 'border-primary-200' : 'border-slate-200'}`}>
      {/* Main Card Content - Clickable */}
      <div
        onClick={handleCardClick}
        className="cursor-pointer p-4 transition-colors hover:bg-slate-50 sm:p-5"
      >
        <div className="flex gap-4">
          <div className={`flex w-14 shrink-0 flex-col items-center justify-center rounded-xl bg-primary-50 text-primary-800 sm:w-16 ${featured ? 'h-[76px]' : 'h-14'}`}>
            <span className="text-[11px] font-semibold uppercase tracking-wide">{format(sessionDate, 'EEE')}</span>
            <span className={`${featured ? 'text-2xl' : 'text-xl'} font-semibold leading-6 tabular-nums`}>{format(sessionDate, 'd')}</span>
            {featured && <span className="text-[11px]">{format(sessionDate, 'MMM')}</span>}
          </div>

          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-slate-950">{session.title}</h3>
            <div className="mt-2 space-y-2 text-sm text-slate-600">
              <div className="flex items-start gap-2">
                <Clock className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                <span className="font-medium tabular-nums">{formatSessionTimeRange(session.start_time, session.end_time, timeFormat)}</span>
              </div>
              <div className="flex min-w-0 items-start gap-2">
                <MapPin className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                <div className="min-w-0 space-y-1">
                  <span className="block break-words [text-wrap:pretty]">{venueName?.trim() || 'Venue to be confirmed'}</span>
                  {courtNumber !== undefined && courtNumber > 0 && (
                    <span className="block whitespace-nowrap font-medium">Court {courtNumber}</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-4">
          <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 text-sm">
            <span className="font-medium text-slate-700">{confirmedCount} of {session.max_players} confirmed</span>
            {session.status === 'cancelled' ? (
              <Badge variant="danger">Cancelled</Badge>
            ) : isDeadlinePassed ? (
              <Badge variant="warning">RSVP Closed</Badge>
            ) : spotsLeft <= 2 && spotsLeft > 0 ? (
              <Badge variant="danger">{spotsLeft} spots left</Badge>
            ) : spotsLeft <= 0 ? (
              <Badge variant="danger">{waitlistedRsvps.length > 0 ? `Full · ${waitlistedRsvps.length} waiting` : 'Full'}</Badge>
            ) : (
              <Badge variant="success">RSVP open</Badge>
            )}
          </div>
        </div>

        <div className="mt-3">
          <PlayerRSVPChips rsvps={session.rsvps ?? []} />
        </div>

        {!isDeadlinePassed && session.status !== 'cancelled' && (
          <div className="mt-4 flex min-w-0 items-center gap-2 text-xs text-amber-800">
            <Timer className="h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" />
            <span>RSVP by {format(new Date(session.rsvp_deadline), 'EEE, d MMM')}</span>
          </div>
        )}
      </div>
    </article>
  );
}

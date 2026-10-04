import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Calendar, Clock, Check, HelpCircle, X, ChevronDown, ChevronUp, MapPin, Timer, Hourglass } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import type { Session } from '../../types';
import Badge from '../ui/Badge';
import Avatar from '../ui/Avatar';
import { displayName } from '../../utils/members';

interface SessionCardProps {
  session: Session;
  venueName?: string;
  featured?: boolean;
}

export default function SessionCard({ session, venueName, featured = false }: SessionCardProps) {
  const navigate = useNavigate();
  const [isExpanded, setIsExpanded] = useState(false);

  const sessionDate = parseISO(session.session_date);
  const isDeadlinePassed = new Date() > new Date(session.rsvp_deadline);

  const confirmedRsvps = session.rsvps?.filter(r => r.status === 'in') || [];
  const waitlistedRsvps = session.rsvps?.filter(r => r.status === 'waitlisted') || [];
  const maybeRsvps = session.rsvps?.filter(r => r.status === 'maybe') || [];
  const declinedRsvps = session.rsvps?.filter(r => r.status === 'out') || [];

  const confirmedCount = confirmedRsvps.length;
  const maybeCount = maybeRsvps.length;
  const declinedCount = declinedRsvps.length;
  const spotsLeft = session.max_players - confirmedCount;
  const capacityPercent = Math.min(100, Math.max(0, (confirmedCount / session.max_players) * 100));

  const handleCardClick = () => {
    navigate(`/sessions/${session.id}`);
  };

  const handleExpandClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setIsExpanded(!isExpanded);
  };

  return (
    <article className={`overflow-hidden rounded-2xl border bg-white shadow-sm ${featured ? 'border-primary-200' : 'border-slate-200'}`}>
      {/* Main Card Content - Clickable */}
      <div
        onClick={handleCardClick}
        className={`${featured ? 'p-5' : 'p-4'} cursor-pointer transition-colors hover:bg-slate-50`}
      >
        <div className="flex gap-4">
          <div className={`flex shrink-0 flex-col items-center justify-center rounded-xl bg-primary-50 text-primary-800 ${featured ? 'h-[76px] w-16' : 'h-14 w-14'}`}>
            <span className="text-[11px] font-semibold uppercase tracking-wide">{format(sessionDate, 'EEE')}</span>
            <span className={`${featured ? 'text-2xl' : 'text-xl'} font-semibold leading-6 tabular-nums`}>{format(sessionDate, 'd')}</span>
            {featured && <span className="text-[11px]">{format(sessionDate, 'MMM')}</span>}
          </div>

          <div className="min-w-0 flex-1">
            <div className="flex items-start justify-between gap-2">
              <h3 className="font-semibold text-slate-950">{session.title}</h3>
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

            <div className="mt-2 space-y-1.5 text-sm text-slate-600">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 shrink-0 text-slate-400" />
                <span>{session.start_time} - {session.end_time}</span>
              </div>
              {venueName && (
                <div className="flex items-center gap-2">
                  <MapPin className="h-4 w-4 shrink-0 text-slate-400" />
                  <span className="truncate">{venueName}</span>
                </div>
              )}
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 shrink-0 text-slate-400" />
                <span>{session.courts} court{session.courts === 1 ? '' : 's'}</span>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-4 rounded-xl bg-slate-100/70 p-3">
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="font-medium text-slate-700">{confirmedCount} of {session.max_players} confirmed</span>
            <span className="text-xs tabular-nums text-slate-500">{spotsLeft > 0 ? `${spotsLeft} left` : 'At capacity'}</span>
          </div>
          <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-200" role="progressbar" aria-label={`${confirmedCount} of ${session.max_players} spots confirmed`} aria-valuemin={0} aria-valuemax={session.max_players} aria-valuenow={confirmedCount}>
            <div className="h-full rounded-full bg-primary-700" style={{ width: `${capacityPercent}%` }} />
          </div>
        </div>

        <div className="mt-3 flex items-center justify-between gap-3">
          {!isDeadlinePassed && session.status !== 'cancelled' ? (
            <div className="flex min-w-0 items-center gap-2 text-xs text-amber-800">
              <Timer className="h-4 w-4 shrink-0 text-amber-600" />
              <span className="truncate">RSVP by {format(new Date(session.rsvp_deadline), 'EEE, d MMM')}</span>
            </div>
          ) : <span />}
          <button
            onClick={handleExpandClick}
            className="flex min-h-11 shrink-0 items-center gap-1 rounded-xl px-2 text-sm font-medium text-slate-500 hover:bg-slate-100 hover:text-slate-800"
            title={isExpanded ? 'Collapse' : 'Expand'}
          >
            {isExpanded ? 'Hide players' : `${maybeCount + declinedCount + confirmedCount + waitlistedRsvps.length} responses`}
            {isExpanded ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>

      {/* Expanded Player List */}
      {isExpanded && (
        <div className="space-y-3 border-t border-slate-100 bg-slate-50 p-4">
          {/* Confirmed Players */}
          {confirmedRsvps.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <Check className="w-3.5 h-3.5 text-green-600" />
                <span className="text-xs font-medium text-slate-600 uppercase tracking-wide">Confirmed</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {confirmedRsvps.map((rsvp) => (
                  <PlayerChip key={rsvp.id} name={displayName(rsvp.user)} picture={rsvp.user?.profile_picture} variant="confirmed" />
                ))}
              </div>
            </div>
          )}

          {/* Maybe Players */}
          {maybeRsvps.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <HelpCircle className="w-3.5 h-3.5 text-amber-600" />
                <span className="text-xs font-medium text-slate-600 uppercase tracking-wide">Maybe</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {maybeRsvps.map((rsvp) => (
                  <PlayerChip key={rsvp.id} name={displayName(rsvp.user)} picture={rsvp.user?.profile_picture} variant="maybe" />
                ))}
              </div>
            </div>
          )}

          {/* Declined Players */}
          {declinedRsvps.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <X className="w-3.5 h-3.5 text-red-600" />
                <span className="text-xs font-medium text-slate-600 uppercase tracking-wide">Can't Make It</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {declinedRsvps.map((rsvp) => (
                  <PlayerChip key={rsvp.id} name={displayName(rsvp.user)} picture={rsvp.user?.profile_picture} variant="declined" />
                ))}
              </div>
            </div>
          )}

          {/* Waitlisted players have responded and should be visible with the response total. */}
          {waitlistedRsvps.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <Hourglass className="w-3.5 h-3.5 text-amber-600" />
                <span className="text-xs font-medium text-slate-600 uppercase tracking-wide">Waitlisted</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {waitlistedRsvps.map((rsvp) => (
                  <PlayerChip key={rsvp.id} name={displayName(rsvp.user)} picture={rsvp.user?.profile_picture} variant="waitlisted" />
                ))}
              </div>
            </div>
          )}

          {/* No RSVPs */}
          {confirmedRsvps.length === 0 && maybeRsvps.length === 0 && declinedRsvps.length === 0 && waitlistedRsvps.length === 0 && (
            <p className="text-sm text-slate-500 text-center py-2">No RSVPs yet</p>
          )}
        </div>
      )}
    </article>
  );
}

function PlayerChip({ name, picture, variant }: { name: string; picture?: string; variant: 'confirmed' | 'maybe' | 'declined' | 'waitlisted' }) {
  const borderColor = variant === 'confirmed'
    ? 'border-green-200 bg-green-50'
    : variant === 'maybe' || variant === 'waitlisted'
      ? 'border-amber-200 bg-amber-50'
      : 'border-red-200 bg-red-50';

  return (
    <div className={`flex items-center gap-1.5 px-2 py-1 rounded-full border ${borderColor}`}>
      <Avatar src={picture} name={name} size="sm" />
      <span className="text-xs font-medium text-slate-700 max-w-[100px] truncate">{name.split(' ')[0]}</span>
    </div>
  );
}

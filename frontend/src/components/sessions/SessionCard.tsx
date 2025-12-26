import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Calendar, Clock, Check, HelpCircle, X, ChevronDown, ChevronUp, MapPin } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import type { Session } from '../../types';
import Badge from '../ui/Badge';
import Avatar from '../ui/Avatar';

interface SessionCardProps {
  session: Session;
  venueName?: string;
}

export default function SessionCard({ session, venueName }: SessionCardProps) {
  const navigate = useNavigate();
  const [isExpanded, setIsExpanded] = useState(false);

  const sessionDate = parseISO(session.session_date);
  const isDeadlinePassed = new Date() > new Date(session.rsvp_deadline);

  const confirmedRsvps = session.rsvps?.filter(r => r.status === 'in') || [];
  const maybeRsvps = session.rsvps?.filter(r => r.status === 'maybe') || [];
  const declinedRsvps = session.rsvps?.filter(r => r.status === 'out') || [];

  const confirmedCount = confirmedRsvps.length;
  const maybeCount = maybeRsvps.length;
  const declinedCount = declinedRsvps.length;
  const spotsLeft = session.max_players - confirmedCount;

  const handleCardClick = () => {
    navigate(`/sessions/${session.id}`);
  };

  const handleExpandClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setIsExpanded(!isExpanded);
  };

  return (
    <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
      {/* Main Card Content - Clickable */}
      <div
        onClick={handleCardClick}
        className="p-4 cursor-pointer hover:bg-slate-50 transition-colors"
      >
        <div className="flex items-start justify-between mb-3">
          <h3 className="font-semibold text-slate-900">{session.title}</h3>
          {session.status === 'cancelled' ? (
            <Badge variant="danger">Cancelled</Badge>
          ) : isDeadlinePassed ? (
            <Badge variant="warning">RSVP Closed</Badge>
          ) : spotsLeft <= 2 && spotsLeft > 0 ? (
            <Badge variant="danger">{spotsLeft} spots left</Badge>
          ) : spotsLeft <= 0 ? (
            <Badge variant="danger">Full</Badge>
          ) : (
            <Badge variant="success">Open</Badge>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-slate-600 mb-3">
          <div className="flex items-center gap-1.5">
            <Calendar className="w-4 h-4 text-primary-500" />
            <span>{format(sessionDate, 'EEE, d MMM')}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <Clock className="w-4 h-4 text-primary-500" />
            <span>{session.start_time} - {session.end_time}</span>
          </div>
          {venueName && (
            <div className="flex items-center gap-1.5">
              <MapPin className="w-4 h-4 text-primary-500" />
              <span className="truncate max-w-[180px]">{venueName}</span>
            </div>
          )}
        </div>

        {/* RSVP Summary Icons */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            {/* Confirmed */}
            <div className="flex items-center gap-1.5" title="Confirmed">
              <div className="w-5 h-5 rounded-full bg-green-100 flex items-center justify-center">
                <Check className="w-3 h-3 text-green-600" />
              </div>
              <span className="text-sm font-medium text-green-700">{confirmedCount}</span>
            </div>

            {/* Maybe */}
            <div className="flex items-center gap-1.5" title="Maybe">
              <div className="w-5 h-5 rounded-full bg-amber-100 flex items-center justify-center">
                <HelpCircle className="w-3 h-3 text-amber-600" />
              </div>
              <span className="text-sm font-medium text-amber-700">{maybeCount}</span>
            </div>

            {/* Declined */}
            <div className="flex items-center gap-1.5" title="Can't make it">
              <div className="w-5 h-5 rounded-full bg-red-100 flex items-center justify-center">
                <X className="w-3 h-3 text-red-600" />
              </div>
              <span className="text-sm font-medium text-red-700">{declinedCount}</span>
            </div>

            <span className="text-xs text-slate-400">/ {session.max_players} max</span>
          </div>

          {/* Expand/Collapse Button */}
          <button
            onClick={handleExpandClick}
            className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
            title={isExpanded ? 'Collapse' : 'Expand'}
          >
            {isExpanded ? (
              <ChevronUp className="w-5 h-5" />
            ) : (
              <ChevronDown className="w-5 h-5" />
            )}
          </button>
        </div>
      </div>

      {/* Expanded Player List */}
      {isExpanded && (
        <div className="border-t border-slate-100 p-4 bg-slate-50 space-y-3">
          {/* Confirmed Players */}
          {confirmedRsvps.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <Check className="w-3.5 h-3.5 text-green-600" />
                <span className="text-xs font-medium text-slate-600 uppercase tracking-wide">Confirmed</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {confirmedRsvps.map((rsvp) => (
                  <PlayerChip key={rsvp.id} name={rsvp.user?.name || ''} picture={rsvp.user?.profile_picture} variant="confirmed" />
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
                  <PlayerChip key={rsvp.id} name={rsvp.user?.name || ''} picture={rsvp.user?.profile_picture} variant="maybe" />
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
                  <PlayerChip key={rsvp.id} name={rsvp.user?.name || ''} picture={rsvp.user?.profile_picture} variant="declined" />
                ))}
              </div>
            </div>
          )}

          {/* No RSVPs */}
          {confirmedRsvps.length === 0 && maybeRsvps.length === 0 && declinedRsvps.length === 0 && (
            <p className="text-sm text-slate-500 text-center py-2">No RSVPs yet</p>
          )}
        </div>
      )}
    </div>
  );
}

function PlayerChip({ name, picture, variant }: { name: string; picture?: string; variant: 'confirmed' | 'maybe' | 'declined' }) {
  const borderColor = variant === 'confirmed'
    ? 'border-green-200 bg-green-50'
    : variant === 'maybe'
      ? 'border-amber-200 bg-amber-50'
      : 'border-red-200 bg-red-50';

  return (
    <div className={`flex items-center gap-1.5 px-2 py-1 rounded-full border ${borderColor}`}>
      <Avatar src={picture} name={name} size="sm" />
      <span className="text-xs font-medium text-slate-700 max-w-[100px] truncate">{name.split(' ')[0]}</span>
    </div>
  );
}

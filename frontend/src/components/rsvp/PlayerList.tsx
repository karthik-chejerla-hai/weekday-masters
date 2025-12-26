import { format } from 'date-fns';
import { Clock, Shield, AlertTriangle } from 'lucide-react';
import type { RSVP } from '../../types';
import Avatar from '../ui/Avatar';
import Badge from '../ui/Badge';

interface PlayerListProps {
  rsvps: RSVP[];
  maxPlayers: number;
  title?: string;
}

export default function PlayerList({ rsvps, maxPlayers, title = 'Confirmed Players' }: PlayerListProps) {
  const confirmedRsvps = rsvps.filter(r => r.status === 'in');
  const maybeRsvps = rsvps.filter(r => r.status === 'maybe');
  const declinedRsvps = rsvps.filter(r => r.status === 'out');

  return (
    <div className="space-y-4">
      {/* Confirmed Players */}
      <div>
        <h3 className="font-semibold text-slate-900 mb-3 flex items-center gap-2">
          {title}
          <Badge variant={confirmedRsvps.length >= maxPlayers ? 'warning' : 'success'}>
            {confirmedRsvps.length} / {maxPlayers}
          </Badge>
        </h3>

        {confirmedRsvps.length === 0 ? (
          <p className="text-slate-500 text-sm">No confirmed players yet</p>
        ) : (
          <div className="space-y-2">
            {confirmedRsvps.map((rsvp, index) => (
              <PlayerItem
                key={rsvp.id}
                rsvp={rsvp}
                position={index + 1}
                isOverCapacity={index >= maxPlayers}
              />
            ))}
          </div>
        )}
      </div>

      {/* Maybe */}
      {maybeRsvps.length > 0 && (
        <div>
          <h4 className="font-medium text-slate-700 mb-2 flex items-center gap-2">
            Maybe
            <Badge variant="warning">{maybeRsvps.length}</Badge>
          </h4>
          <div className="space-y-2">
            {maybeRsvps.map((rsvp) => (
              <PlayerItem key={rsvp.id} rsvp={rsvp} variant="maybe" />
            ))}
          </div>
        </div>
      )}

      {/* Can't Make It */}
      {declinedRsvps.length > 0 && (
        <div>
          <h4 className="font-medium text-slate-700 mb-2 flex items-center gap-2">
            Can't Make It
            <Badge variant="danger">{declinedRsvps.length}</Badge>
          </h4>
          <div className="space-y-2">
            {declinedRsvps.map((rsvp) => (
              <PlayerItem key={rsvp.id} rsvp={rsvp} variant="out" />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

interface PlayerItemProps {
  rsvp: RSVP;
  position?: number;
  isOverCapacity?: boolean;
  variant?: 'confirmed' | 'maybe' | 'out';
}

function PlayerItem({ rsvp, position, isOverCapacity, variant = 'confirmed' }: PlayerItemProps) {
  const user = rsvp.user;
  if (!user) return null;

  const bgColor = isOverCapacity
    ? 'bg-amber-50'
    : variant === 'maybe'
      ? 'bg-amber-50/50'
      : variant === 'out'
        ? 'bg-red-50/50'
        : 'bg-white';

  return (
    <div className={`flex items-center gap-3 p-2 rounded-lg ${bgColor}`}>
      {position && (
        <span className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium ${
          isOverCapacity ? 'bg-amber-200 text-amber-800' : 'bg-primary-100 text-primary-700'
        }`}>
          {position}
        </span>
      )}

      <Avatar src={user.profile_picture} name={user.name} size="sm" />

      <div className="flex-1 min-w-0">
        <p className="font-medium text-slate-900 truncate">{user.name}</p>
        <p className="text-xs text-slate-500 flex items-center gap-1">
          <Clock className="w-3 h-3" />
          {format(new Date(rsvp.rsvp_timestamp), 'MMM d, h:mm a')}
        </p>
      </div>

      <div className="flex items-center gap-1">
        {rsvp.added_by_admin && (
          <span title="Added by admin">
            <Shield className="w-4 h-4 text-primary-500" />
          </span>
        )}
        {rsvp.is_late_rsvp && (
          <span title="Late RSVP">
            <AlertTriangle className="w-4 h-4 text-amber-500" />
          </span>
        )}
        {isOverCapacity && (
          <Badge variant="warning">Overflow</Badge>
        )}
      </div>
    </div>
  );
}

import { useState } from 'react';
import axios from 'axios';
import { format } from 'date-fns';
import { Clock, Shield, AlertTriangle, Pencil, Trash2, Loader2 } from 'lucide-react';
import type { GuestRSVP, PlayerResponse } from '../../types';
import { isGuest, playerName } from '../../utils/player-responses';
import Avatar from '../ui/Avatar';
import Badge from '../ui/Badge';
import { displayName } from '../../utils/members';

interface PlayerListProps {
  rsvps: PlayerResponse[];
  maxPlayers: number;
  title?: string;
  onEditGuest?: (guest: GuestRSVP) => void;
  onRemoveGuest?: (guest: GuestRSVP) => Promise<void>;
}

export default function PlayerList({ rsvps, maxPlayers, title = 'Confirmed Players', onEditGuest, onRemoveGuest }: PlayerListProps) {
  const confirmedRsvps = rsvps.filter(r => r.status === 'in');
  const waitlistedRsvps = rsvps.filter(r => r.status === 'waitlisted');
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
                onEditGuest={onEditGuest}
                onRemoveGuest={onRemoveGuest}
                position={index + 1}
                isOverCapacity={index >= maxPlayers}
              />
            ))}
          </div>
        )}
      </div>

      {/* Waitlist */}
      {waitlistedRsvps.length > 0 && (
        <div>
          <h4 className="font-medium text-slate-700 mb-2 flex items-center gap-2">
            Waitlist
            <Badge variant="warning">{waitlistedRsvps.length}</Badge>
          </h4>
          <div className="space-y-2">
            {waitlistedRsvps.map((rsvp, index) => (
              <PlayerItem
                key={rsvp.id}
                rsvp={rsvp}
                onEditGuest={onEditGuest}
                position={index + 1}
                variant="waitlisted"
              />
            ))}
          </div>
        </div>
      )}

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
              <PlayerItem key={rsvp.id} rsvp={rsvp} variant="out" onEditGuest={onEditGuest} onRemoveGuest={onRemoveGuest} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

interface PlayerItemProps {
  rsvp: PlayerResponse;
  onEditGuest?: (guest: GuestRSVP) => void;
  onRemoveGuest?: (guest: GuestRSVP) => Promise<void>;
  position?: number;
  isOverCapacity?: boolean;
  variant?: 'confirmed' | 'maybe' | 'out' | 'waitlisted';
}

function PlayerItem({ rsvp, position, isOverCapacity, variant = 'confirmed', onEditGuest, onRemoveGuest }: PlayerItemProps) {
  const [removing, setRemoving] = useState(false);
  const [removeError, setRemoveError] = useState('');
  const guest = isGuest(rsvp);
  const user = guest ? undefined : rsvp.user;
  const name = playerName(rsvp);

  const isWaitlisted = variant === 'waitlisted';

  const removeGuest = async () => {
    if (!guest || !onRemoveGuest || removing) return;
    setRemoving(true);
    setRemoveError('');
    try {
      await onRemoveGuest(rsvp);
    } catch (error) {
      setRemoveError(axios.isAxiosError(error) && typeof error.response?.data?.error === 'string'
        ? error.response.data.error : `Could not remove ${name}. Please try again.`);
    } finally {
      setRemoving(false);
    }
  };

  const bgColor = isOverCapacity || isWaitlisted
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
          isOverCapacity || isWaitlisted ? 'bg-amber-200 text-amber-800' : 'bg-primary-100 text-primary-700'
        }`}>
          {position}
        </span>
      )}

      <Avatar src={user?.profile_picture} name={name} size="sm" />

      <div className="flex-1 min-w-0">
        <p className="flex flex-wrap items-baseline gap-x-2 font-medium text-slate-900"><span className="max-w-full truncate">{name}</span>{guest && <span className="text-xs font-normal text-slate-500">Guest</span>}</p>
        {guest && <p className="text-xs text-slate-500">Charged to {displayName(rsvp.charged_to_user) || 'member'}</p>}
        <p className="text-xs text-slate-500 flex items-center gap-1">
          <Clock className="w-3 h-3" />
          {format(new Date(rsvp.rsvp_timestamp), 'MMM d, h:mm a')}
        </p>
        {removeError && <p role="alert" className="mt-1 text-sm text-red-700">{removeError}</p>}
      </div>

      <div className="flex items-center gap-1">
        {guest && onEditGuest && <button type="button" className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-primary-700 hover:bg-primary-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-700 disabled:opacity-50"
          aria-label={`Edit ${name}'s RSVP`} title={`Edit ${name}'s RSVP`} disabled={removing} onClick={() => onEditGuest(rsvp)}>
          <Pencil aria-hidden="true" className="h-4 w-4" />
        </button>}
        {guest && onRemoveGuest && <button type="button" className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-red-700 hover:bg-red-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-700 disabled:opacity-50"
          aria-label={`Remove ${name}'s RSVP`} title={`Remove ${name}'s RSVP`} disabled={removing} onClick={removeGuest}>
          {removing ? <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Trash2 aria-hidden="true" className="h-4 w-4" />}
        </button>}
        {(guest || rsvp.added_by_admin) && (
          <span title="Added by admin">
            <Shield className="w-4 h-4 text-primary-500" />
          </span>
        )}
        {!guest && rsvp.is_late_rsvp && (
          <span title="Late RSVP">
            <AlertTriangle className="w-4 h-4 text-amber-500" />
          </span>
        )}
        {isOverCapacity && (
          <Badge variant="warning">Overflow</Badge>
        )}
        {isWaitlisted && (
          <Badge variant="warning">Waiting</Badge>
        )}
      </div>
    </div>
  );
}

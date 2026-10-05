import { Hourglass } from 'lucide-react';
import type { RSVP, RSVPStatus } from '../../types';
import { displayName } from '../../utils/members';
import Avatar from '../ui/Avatar';

const statusOrder: RSVPStatus[] = ['in', 'maybe', 'out', 'waitlisted'];
const statusStyles: Record<RSVPStatus, { label: string; className: string }> = {
  in: { label: 'Confirmed', className: 'border-green-300 bg-green-50' },
  maybe: { label: 'Tentative', className: 'border-amber-300 bg-amber-50' },
  out: { label: 'Not attending', className: 'border-red-300 bg-red-50' },
  waitlisted: { label: 'Waitlisted', className: 'border-amber-300 bg-amber-50' },
};

export default function PlayerRSVPChips({ rsvps }: { rsvps: RSVP[] }) {
  if (rsvps.length === 0) {
    return <p className="text-sm text-slate-500">No RSVPs yet</p>;
  }

  return (
    <ul aria-label="Player responses" className="flex flex-wrap gap-2">
      {statusOrder.flatMap(status => rsvps.filter(rsvp => rsvp.status === status)).map(rsvp => {
        const name = displayName(rsvp.user) || 'Player';
        const { label, className } = statusStyles[rsvp.status];

        return (
          <li
            key={rsvp.id}
            aria-label={`${name}, ${label}`}
            title={`${name}, ${label}`}
            className={`flex max-w-full items-center gap-1.5 rounded-full border px-2 py-1 ${className}`}
          >
            <span aria-hidden="true" className="shrink-0">
              <Avatar src={rsvp.user?.profile_picture} name={name} size="sm" />
            </span>
            <span className="max-w-[100px] truncate text-xs font-medium text-slate-700">{name}</span>
            {rsvp.status === 'waitlisted' && <Hourglass className="h-3.5 w-3.5 shrink-0 text-amber-600" aria-hidden="true" />}
          </li>
        );
      })}
    </ul>
  );
}

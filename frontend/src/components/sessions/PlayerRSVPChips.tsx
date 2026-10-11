import type { RSVP, RSVPStatus } from '../../types';
import { displayName } from '../../utils/members';

const responses: Record<RSVPStatus, { label: string; className: string }> = {
  in: { label: 'IN', className: 'border-green-600 bg-green-50 text-green-900' },
  maybe: { label: 'MAYBE', className: 'border-yellow-500 bg-yellow-50 text-yellow-900' },
  out: { label: 'OUT', className: 'border-red-600 bg-red-50 text-red-900' },
  waitlisted: { label: 'WAITLISTED', className: 'border-yellow-500 bg-yellow-50 text-yellow-900' },
};
const statusOrder: RSVPStatus[] = ['in', 'maybe', 'out', 'waitlisted'];

export default function PlayerRSVPChips({ rsvps }: { rsvps: RSVP[] }) {
  if (rsvps.length === 0) {
    return <p className="text-sm text-slate-500">No RSVPs yet</p>;
  }

  return (
    <div>
      <p className="mb-1 text-xs font-medium text-slate-500">Player responses</p>
      <ul aria-label="Player responses" className="flex flex-wrap gap-1.5">
        {statusOrder.flatMap(status => rsvps.filter(rsvp => rsvp.status === status)).map(rsvp => {
          const { label, className } = responses[rsvp.status];
          return (
            <li key={rsvp.id} className={`max-w-full break-words rounded-full border-2 px-2 py-1 text-xs ${className}`}>
              <span className="font-medium">{displayName(rsvp.user) || 'Player'}</span>
              <span className="ml-1.5 text-[10px] font-semibold">{label}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

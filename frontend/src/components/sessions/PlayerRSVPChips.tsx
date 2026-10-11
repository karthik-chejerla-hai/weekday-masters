import type { RSVP } from '../../types';
import { displayName } from '../../utils/members';

export default function PlayerRSVPChips({ rsvps }: { rsvps: RSVP[] }) {
  const confirmed = rsvps.filter(rsvp => rsvp.status === 'in');
  if (confirmed.length === 0) {
    return <p className="text-sm text-slate-500">No confirmed players yet</p>;
  }

  return (
    <div>
      <p className="mb-1 text-xs font-medium text-slate-500">Confirmed players</p>
      <ul aria-label="Confirmed players" className="flex flex-wrap gap-1.5">
        {confirmed.map(rsvp => (
          <li key={rsvp.id} className="max-w-full break-words rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-700">
            {displayName(rsvp.user) || 'Player'}
          </li>
        ))}
      </ul>
    </div>
  );
}

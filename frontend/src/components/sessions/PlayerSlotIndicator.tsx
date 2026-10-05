import { Plus } from 'lucide-react';
import Avatar from '../ui/Avatar';

interface PlayerSlotIndicatorProps {
  capacity: number;
  reservedCount: number;
  players?: Array<{ id: string; name: string; picture?: string }>;
}

export default function PlayerSlotIndicator({ capacity, reservedCount, players = [] }: PlayerSlotIndicatorProps) {
  const slots = Number.isFinite(capacity) ? Math.max(0, Math.floor(capacity)) : 0;
  const reserved = Number.isNaN(reservedCount) ? 0 : Math.min(slots, Math.max(0, Math.floor(reservedCount)));

  return (
    <div
      role="group"
      aria-label={`${reserved} of ${slots} player slots reserved; ${slots - reserved} available`}
      className="flex flex-wrap items-center justify-center gap-3 py-4 sm:gap-4"
    >
      {Array.from({ length: reserved }, (_, index) => {
        const player = players[index];
        const name = player?.name || 'Confirmed player';
        const label = player?.name ? `${name}, confirmed` : name;

        return (
          <span
            key={player?.id ?? `reserved-${index}`}
            role="img"
            aria-label={label}
            title={label}
            className="shrink-0 rounded-full ring-2 ring-green-500 ring-offset-2 ring-offset-white"
          >
            <span aria-hidden="true">
              <Avatar src={player?.picture} name={player?.name || ''} size="sm" />
            </span>
          </span>
        );
      })}
      {Array.from({ length: slots - reserved }, (_, index) => (
        <span
          key={`available-${index}`}
          role="img"
          aria-label="Available player slot"
          title="Available player slot"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-dashed border-slate-300 bg-slate-50"
        >
          <Plus className="h-3.5 w-3.5 text-slate-300" aria-hidden="true" />
        </span>
      ))}
    </div>
  );
}

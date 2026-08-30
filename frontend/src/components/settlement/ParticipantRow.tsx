import { X } from 'lucide-react';
import Avatar from '../ui/Avatar';
import { formatCents } from '../money/format';
import type { ChargeLine } from '../../types';

export interface EditableLine {
  user_id: string;
  name: string;
  profile_picture?: string;
  guest_name?: string;
  in_base: boolean;
  in_extra: boolean;
  comped: boolean;
}

interface ParticipantRowProps {
  line: EditableLine;
  /** The costed line from the latest preview, if it has come back yet. */
  costed?: ChargeLine;
  hasExtraHour: boolean;
  onChange: (next: EditableLine) => void;
  onRemove: () => void;
}

/**
 * One participant on the settlement form.
 *
 * The controls are framed positively — "stayed for the extra hour" is ticked by
 * default when the session ran one — because a negative default reads backwards.
 * Unticking both bands removes them entirely, which is what "did not turn up and
 * is not being charged" means.
 */
export default function ParticipantRow({
  line,
  costed,
  hasExtraHour,
  onChange,
  onRemove,
}: ParticipantRowProps) {
  const isGuest = Boolean(line.guest_name);

  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <Avatar src={line.profile_picture} name={line.guest_name || line.name} size="sm" />

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-slate-800">
          {isGuest ? line.guest_name : line.name}
        </p>
        {isGuest && <p className="text-xs text-slate-400">guest of {line.name}</p>}
      </div>

      {hasExtraHour && (
        <label className="flex items-center gap-1.5 text-xs text-slate-600">
          <input
            type="checkbox"
            className="rounded border-slate-300 text-primary-600 focus:ring-primary-500"
            checked={line.in_extra}
            onChange={(e) => onChange({ ...line, in_extra: e.target.checked })}
          />
          Extra hour
        </label>
      )}

      <label className="flex items-center gap-1.5 text-xs text-slate-600">
        <input
          type="checkbox"
          className="rounded border-slate-300 text-primary-600 focus:ring-primary-500"
          checked={line.comped}
          onChange={(e) => onChange({ ...line, comped: e.target.checked })}
        />
        Comp
      </label>

      <span className="w-16 text-right text-sm font-medium text-slate-700 tabular-nums">
        {costed ? formatCents(costed.amount_cents) : '—'}
      </span>

      <button
        type="button"
        onClick={onRemove}
        className="p-1 text-slate-400 hover:text-red-600 rounded"
        aria-label={`Remove ${line.guest_name || line.name}`}
      >
        <X className="w-4 h-4" />
      </button>
    </li>
  );
}

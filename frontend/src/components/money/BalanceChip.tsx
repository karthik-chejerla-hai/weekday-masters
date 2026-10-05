import type { BalanceState } from '../../types';
import { formatCents } from './format';
import ValueBadge from '../ui/ValueBadge';

const TONE: Record<BalanceState, string> = {
  ok: 'bg-primary-50 text-primary-700 border-primary-200',
  low: 'bg-secondary-50 text-secondary-700 border-secondary-300',
  negative: 'bg-red-50 text-red-700 border-red-200',
};

interface BalanceChipProps {
  cents: number;
  state: BalanceState;
  /** Compact form for tight layouts with a surrounding label. */
  compact?: boolean;
  title?: string;
  /** Joined label/value badge where the amount has no surrounding label. */
  label?: string;
}

/**
 * A member's balance, colour-coded.
 *
 * In the header this is doing quiet work: someone whose chip is red every time
 * they open the app tends not to need an email about it.
 */
export default function BalanceChip({ cents, state, compact = false, title, label }: BalanceChipProps) {
  const description = title ?? (state === 'negative' ? 'You owe the club' : undefined);
  if (label) {
    return (
      <ValueBadge
        label={label}
        value={formatCents(cents)}
        tone={state === 'negative' ? 'negative' : state === 'low' ? 'warning' : 'positive'}
        title={description}
      />
    );
  }
  return (
    <span
      className={`inline-flex items-center rounded-full border font-medium tabular-nums ${TONE[state]} ${
        compact ? 'px-2 py-0.5 text-xs' : 'px-3 py-1 text-sm'
      }`}
      title={description}
    >
      {formatCents(cents)}
    </span>
  );
}

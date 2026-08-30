import type { BalanceState } from '../../types';

/**
 * Format integer cents as AUD.
 *
 * This is the only place in the frontend that divides by 100. Money arrives
 * from the API as integer cents and stays that way through every comparison and
 * every sum; converting to a float any earlier is how rounding errors start.
 */
export function formatCents(cents: number): string {
  const negative = cents < 0;
  const absolute = Math.abs(cents);
  const dollars = Math.trunc(absolute / 100);
  const remainder = absolute % 100;

  const formatted = `$${dollars.toLocaleString('en-AU')}.${remainder.toString().padStart(2, '0')}`;
  return negative ? `-${formatted}` : formatted;
}

/**
 * Convert a dollars-and-cents input string to integer cents.
 *
 * Done once, at the edge, so nothing downstream ever holds a float.
 */
export function parseDollarsToCents(value: string): number {
  const parsed = Number.parseFloat(value);
  if (!Number.isFinite(parsed)) return NaN;
  return Math.round(parsed * 100);
}

/**
 * Derive the display state when the server has not already said.
 *
 * The authoritative rule lives on the server — `GET /accounts/me` returns a
 * state — so this is only used for other people's balances on the balances
 * list, where the threshold comes from the same club setting.
 */
export function balanceState(cents: number, lowThresholdCents: number): BalanceState {
  if (cents < 0) return 'negative';
  if (cents < lowThresholdCents) return 'low';
  return 'ok';
}

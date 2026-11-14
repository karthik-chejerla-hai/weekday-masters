import type { BalanceNudgeResult } from '../types';
export function balanceNudgeMessage(result: BalanceNudgeResult, name: string): string {
  if (result.delivery_state === 'pending' || result.delivery_state === 'claimed') return `Push nudge queued for ${name}.`;
  if (result.push_sent) return `Push accepted for ${name}; this does not confirm they saw it.`;
  if (result.delivery_state === 'uncertain') return `Push outcome is uncertain for ${name}. The reminder is recorded; avoid resending.`;
  if (result.delivery_state === 'failed' || result.delivery_state === 'expired') return `Push could not be completed for ${name}. The reminder is recorded in Rally.`;
  return `${name}'s reminder is recorded in Rally; push is suppressed or unavailable.`;
}

import type { User } from '../types';

/**
 * What the club calls a member: their nickname when they have one, otherwise
 * the name their sign-in provider gave us.
 *
 * The backend applies the same rule when it names ledger accounts and
 * settlement lines, so a nickname set on the admin screen shows up everywhere
 * rather than leaving two names for one person on screen at once.
 */
export function displayName(user?: Pick<User, 'name' | 'nickname'> | null): string {
  if (!user) return '';
  return user.nickname?.trim() || user.name;
}

/**
 * True for a member an admin added who has never signed in.
 *
 * Their row carries a placeholder in place of an Auth0 subject, which is what
 * makes their email still safe to correct — see models.InvitePlaceholderPrefix.
 */
export function isPendingInvite(user: Pick<User, 'auth0_id'>): boolean {
  return user.auth0_id.startsWith('invite:');
}

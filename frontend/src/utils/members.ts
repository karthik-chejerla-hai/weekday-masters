import type { User } from '../types';

/**
 * What the club calls a member: the nickname they chose, otherwise their first
 * name.
 *
 * Must stay in step with `models.User.DisplayName` on the backend, which names
 * ledger accounts and settlement lines by the same rule — otherwise one person
 * appears under two names on the same screen.
 */
export function displayName(user?: Pick<User, 'name' | 'nickname'> | null): string {
  if (!user) return '';
  return user.nickname?.trim() || firstName(user.name);
}

/**
 * The leading word of a full name, falling back to the whole string when there
 * is nothing to split — a mononym is a first name.
 */
export function firstName(name: string): string {
  const trimmed = name.trim();
  return trimmed.split(' ')[0] || trimmed;
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

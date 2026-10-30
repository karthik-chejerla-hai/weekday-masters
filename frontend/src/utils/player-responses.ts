import type { GuestRSVP, PlayerResponse, Session } from '../types';
import { displayName } from './members';

export function isGuest(response: PlayerResponse): response is GuestRSVP {
  return 'guest_name' in response;
}

export function playerName(response: PlayerResponse): string {
  return isGuest(response) ? response.guest_name : displayName(response.user) || 'Player';
}

export function sessionResponses(session: Session): PlayerResponse[] {
  return [...session.rsvps ?? [], ...session.guest_rsvps ?? []]
    .sort((a, b) => a.rsvp_timestamp.localeCompare(b.rsvp_timestamp) || a.id.localeCompare(b.id));
}

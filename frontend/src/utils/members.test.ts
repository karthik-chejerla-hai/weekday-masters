import { describe, it, expect } from 'vitest';
import { displayName, isPendingInvite } from './members';

describe('displayName', () => {
  it('prefers the nickname the club gave them', () => {
    expect(displayName({ name: 'Priya Raman', nickname: 'Pri' })).toBe('Pri');
  });

  it('falls back to the provider name when there is no nickname', () => {
    expect(displayName({ name: 'Priya Raman', nickname: '' })).toBe('Priya Raman');
  });

  it('treats a whitespace-only nickname as absent', () => {
    expect(displayName({ name: 'Priya Raman', nickname: '   ' })).toBe('Priya Raman');
  });

  it('renders nothing for a missing user, so a partial RSVP does not print "undefined"', () => {
    expect(displayName(null)).toBe('');
    expect(displayName(undefined)).toBe('');
  });
});

describe('isPendingInvite', () => {
  it('recognises the placeholder subject an invite carries', () => {
    expect(isPendingInvite({ auth0_id: 'invite:6f1c9e2a-0000-4000-8000-000000000000' })).toBe(true);
  });

  it('does not mistake a real Auth0 subject for one', () => {
    expect(isPendingInvite({ auth0_id: 'google-oauth2|110358' })).toBe(false);
  });
});

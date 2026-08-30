import { describe, it, expect } from 'vitest';
import { displayName, firstName, isPendingInvite } from './members';

describe('displayName', () => {
  it('prefers the nickname the club gave them', () => {
    expect(displayName({ name: 'Priya Raman', nickname: 'Pri' })).toBe('Pri');
  });

  it('falls back to the first name, which is what people are called courtside', () => {
    expect(displayName({ name: 'Priya Raman', nickname: '' })).toBe('Priya');
  });

  it('treats a whitespace-only nickname as absent', () => {
    expect(displayName({ name: 'Priya Raman', nickname: '   ' })).toBe('Priya');
  });

  it('leaves a mononym whole, since it is already a first name', () => {
    expect(displayName({ name: 'Ronaldinho', nickname: '' })).toBe('Ronaldinho');
  });

  it('agrees with the backend on middle names and stray whitespace', () => {
    expect(displayName({ name: 'Anna Maria de Souza', nickname: '' })).toBe('Anna');
    expect(displayName({ name: '  Wei Zhang ', nickname: '' })).toBe('Wei');
  });

  it('renders nothing for a missing user, so a partial RSVP does not print "undefined"', () => {
    expect(displayName(null)).toBe('');
    expect(displayName(undefined)).toBe('');
  });
});

describe('firstName', () => {
  it('takes the leading word', () => {
    expect(firstName('Priya Raman')).toBe('Priya');
  });

  it('returns the whole string when there is nothing to split', () => {
    expect(firstName('Ronaldinho')).toBe('Ronaldinho');
    expect(firstName('')).toBe('');
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

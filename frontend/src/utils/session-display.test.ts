import { describe, expect, it } from 'vitest';
import type { Session } from '../types';
import { formatSessionTime, formatSessionTimeRange, formatSessionVenue, nextScheduledSession } from './session-display';

describe('session display', () => {
  it.each([
    ['00:00', '12:00 AM'], ['00:30', '12:30 AM'], ['09:05', '9:05 AM'],
    ['12:00', '12:00 PM'], ['12:45', '12:45 PM'], ['20:00', '8:00 PM'], ['23:59', '11:59 PM'],
  ])('formats %s as %s without timezone conversion', (input, expected) => {
    expect(formatSessionTime(input, '12h')).toBe(expected);
    expect(formatSessionTime(input, '24h')).toBe(input);
    expect(formatSessionTime(input)).toBe(input);
  });

  it('keeps both AM and PM when a session crosses noon or midnight', () => {
    expect(formatSessionTimeRange('11:00', '13:00', '12h')).toBe('11:00 AM - 1:00 PM');
    expect(formatSessionTimeRange('23:00', '01:00', '12h')).toBe('11:00 PM - 1:00 AM');
  });

  it('shows the venue and actual court number without inventing an unset court', () => {
    expect(formatSessionVenue('BadmintonWorx Norwest', 8)).toBe('BadmintonWorx Norwest - Court 8');
    expect(formatSessionVenue('BadmintonWorx Norwest', 0)).toBe('BadmintonWorx Norwest');
    expect(formatSessionVenue('', 8)).toBe('Court 8');
    expect(formatSessionVenue()).toBe('Venue to be confirmed');
  });
});

describe('next scheduled session', () => {
  const now = Date.parse('2026-10-05T19:10:00+11:00');
  const session = (id: string, startsAt?: string, status: Session['status'] = 'open') => ({
    id, starts_at: startsAt, status,
    session_date: startsAt?.slice(0, 10) ?? '2026-10-05',
    start_time: startsAt?.slice(11, 16) ?? '18:00',
  } as Session);
  const cancelled = session('cancelled', '2026-10-05T18:00:00+11:00', 'cancelled');

  it('skips games that have started when cancelling an ongoing game', () => {
    const next = session('next', '2026-10-12T18:00:00+11:00', 'closed');
    expect(nextScheduledSession([
      session('ongoing', '2026-10-05T19:00:00+11:00'),
      session('starting-now', '2026-10-05T19:10:00+11:00'),
      session('also-cancelled', '2026-10-06T18:00:00+11:00', 'cancelled'),
      next,
    ], cancelled, now)).toBe(next);
  });

  it('requires a start after both now and the cancelled game', () => {
    const futureCancellation = session('future', '2026-10-12T18:00:00+11:00', 'cancelled');
    const next = session('next', '2026-10-19T18:00:00+11:00');
    expect(nextScheduledSession([
      session('before-cancellation', '2026-10-10T18:00:00+11:00'),
      session('same-time', futureCancellation.starts_at),
      futureCancellation,
      next,
    ], futureCancellation, now)).toBe(next);
  });

  it('orders resolved instants across timezone offsets', () => {
    const next = session('next', '2026-10-05T20:00:00+11:00');
    expect(nextScheduledSession([
      session('later', '2026-10-05T10:00:00Z'),
      next,
    ], cancelled, now)).toBe(next);
  });

  it('returns no next game when only started or unresolved games remain', () => {
    expect(nextScheduledSession([
      session('started', '2026-10-05T19:00:00+11:00'),
      session('missing'),
      session('invalid', 'invalid'),
    ], cancelled, now)).toBeUndefined();
  });
});

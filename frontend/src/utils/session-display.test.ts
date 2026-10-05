import { describe, expect, it } from 'vitest';
import { formatSessionTime, formatSessionTimeRange, formatSessionVenue } from './session-display';

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

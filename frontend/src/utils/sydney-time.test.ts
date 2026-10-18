import { describe, expect, it } from 'vitest';
import { parseSydneyDateTime } from './sydney-time';

describe('Sydney date and time input', () => {
  it.each(['2027-10-03T02:00', '2027-10-03T02:30', '2027-10-03T02:59:59', '2027-02-30T18:00', '', 'invalid'])('rejects %s', value => {
    expect(parseSydneyDateTime(value)).toBeNull();
  });

  it.each([
    ['2027-01-10T18:00', '2027-01-10T07:00:00.000Z'],
    ['2027-07-11T18:00', '2027-07-11T08:00:00.000Z'],
    ['2027-10-03T01:59:59', '2027-10-02T15:59:59.000Z'],
    ['2027-10-03T03:00', '2027-10-02T16:00:00.000Z'],
  ])('preserves the selected Sydney time %s', (value, expected) => {
    expect(parseSydneyDateTime(value)?.toISOString()).toBe(expected);
  });
});

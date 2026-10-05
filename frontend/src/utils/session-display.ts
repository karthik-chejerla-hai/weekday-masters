import type { TimeFormat } from '../types';

// Session times are already Sydney wall-clock times. Do not apply the
// browser's timezone to them when changing the display format.
export function formatSessionTime(value: string, timeFormat: TimeFormat = '24h'): string {
  const match = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(value);
  if (!match || timeFormat !== '12h') return value;
  const hour = Number(match[1]);
  return `${hour % 12 || 12}:${match[2]} ${hour < 12 ? 'AM' : 'PM'}`;
}

export function formatSessionTimeRange(start: string, end: string, timeFormat?: TimeFormat): string {
  return `${formatSessionTime(start, timeFormat)} - ${formatSessionTime(end, timeFormat)}`;
}

export function formatSessionVenue(venueName?: string, courtNumber?: number): string {
  return [venueName?.trim(), courtNumber && courtNumber > 0 ? `Court ${courtNumber}` : '']
    .filter(Boolean).join(' - ') || 'Venue to be confirmed';
}

import { formatInTimeZone, fromZonedTime } from 'date-fns-tz';

/** Reject clock times that do not exist during Sydney's daylight saving jump. */
export function parseSydneyDateTime(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/.test(value)) return null;
  const normalized = value.length === 16 ? `${value}:00` : value;
  const instant = fromZonedTime(normalized, 'Australia/Sydney');
  if (Number.isNaN(instant.getTime())) return null;
  return formatInTimeZone(instant, 'Australia/Sydney', "yyyy-MM-dd'T'HH:mm:ss") === normalized
    ? instant
    : null;
}

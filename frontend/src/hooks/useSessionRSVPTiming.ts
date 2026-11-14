import { useEffect, useState } from 'react';
import type { Session } from '../types';

/** Keep stored RSVP boundaries current, including pages resumed from the background. */
export function useSessionRSVPTiming(session?: Session | null) {
  const startsAt = Date.parse(session?.starts_at ?? '');
  const deadline = Date.parse(session?.rsvp_deadline ?? '');
  const [now, setNow] = useState(Date.now);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const refresh = () => {
      const current = Date.now();
      setNow(current);
      clearTimeout(timer);
      // The deadline is inclusive; IN requests close exactly at the start.
      const next = [deadline + 1, startsAt].filter(at => Number.isFinite(at) && at > current);
      if (next.length) {
        timer = setTimeout(refresh, Math.min(Math.min(...next) - current, 2_147_483_647));
      }
    };
    refresh();
    window.addEventListener('focus', refresh);
    document.addEventListener('visibilitychange', refresh);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('focus', refresh);
      document.removeEventListener('visibilitychange', refresh);
    };
  }, [startsAt, deadline]);

  return {
    startsAt,
    now,
    isStartUnavailable: !Number.isFinite(startsAt),
    isDeadlinePassed: now > deadline,
    canJoin: session?.status === 'open' && now < startsAt,
  };
}

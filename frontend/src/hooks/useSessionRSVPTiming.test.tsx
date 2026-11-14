import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '../types';
import { useSessionRSVPTiming } from './useSessionRSVPTiming';

afterEach(() => vi.useRealTimers());

function upcoming(deadline: number, start: number): Session {
  return { status: 'open', rsvp_deadline: new Date(deadline).toISOString(), starts_at: new Date(start).toISOString() } as Session;
}

describe('stored RSVP time boundaries', () => {
  it('locks confirmed responses after the inclusive deadline and closes IN exactly at start', () => {
    vi.useFakeTimers();
    const now = Date.now();
    const { result, unmount } = renderHook(() => useSessionRSVPTiming(upcoming(now + 1000, now + 2000)));
    expect(result.current.isDeadlinePassed).toBe(false);
    expect(result.current.canJoin).toBe(true);
    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.isDeadlinePassed).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.isDeadlinePassed).toBe(true);
    expect(result.current.canJoin).toBe(true);
    act(() => vi.advanceTimersByTime(999));
    expect(result.current.canJoin).toBe(false);
    unmount();
  });

  it('reschedules when an admin changes the stored start', () => {
    vi.useFakeTimers();
    const now = Date.now();
    const { result, rerender, unmount } = renderHook(({ session }) => useSessionRSVPTiming(session), {
      initialProps: { session: upcoming(now - 1000, now + 1000) },
    });
    rerender({ session: upcoming(now - 1000, now + 2000) });
    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.canJoin).toBe(true);
    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.canJoin).toBe(false);
    unmount();
  });

  it('handles sessions further away than the browser timeout limit', () => {
    vi.useFakeTimers();
    const now = Date.now();
    const offset = 40 * 86400000;
    const { result, unmount } = renderHook(() => useSessionRSVPTiming(upcoming(now + offset - 1000, now + offset)));
    act(() => vi.advanceTimersByTime(offset - 1));
    expect(result.current.canJoin).toBe(true);
    expect(result.current.isDeadlinePassed).toBe(true);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.canJoin).toBe(false);
    unmount();
  });
});

import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useApi, useApiMutation } from './useApi';

describe('useApi Hook', () => {
  it('initializes with default state', () => {
    const { result } = renderHook(() => useApi<string>());
    expect(result.current.data).toBeNull();
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('handles successful API execution', async () => {
    const { result } = renderHook(() => useApi<string>());

    await act(async () => {
      const data = await result.current.execute(async () => 'success_data');
      expect(data).toBe('success_data');
    });

    expect(result.current.data).toBe('success_data');
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('handles failed API execution', async () => {
    const { result } = renderHook(() => useApi<string>());

    await act(async () => {
      try {
        await result.current.execute(async () => {
          throw new Error('API Error');
        });
      } catch {
        // Expected
      }
    });

    expect(result.current.data).toBeNull();
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBe('API Error');
  });
});

describe('useApiMutation Hook', () => {
  it('handles successful mutation', async () => {
    const { result } = renderHook(() => useApiMutation<number, [number, number]>());

    await act(async () => {
      const sum = await result.current.mutate(async (a, b) => a + b, 2, 3);
      expect(sum).toBe(5);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('handles mutation errors', async () => {
    const { result } = renderHook(() => useApiMutation<void, []>());

    await act(async () => {
      try {
        await result.current.mutate(async () => {
          throw new Error('Mutation failed');
        });
      } catch {
        // Expected
      }
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBe('Mutation failed');
  });
});

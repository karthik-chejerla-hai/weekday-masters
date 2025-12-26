import { useState, useCallback } from 'react';

interface UseApiState<T> {
  data: T | null;
  isLoading: boolean;
  error: string | null;
}

export function useApi<T>() {
  const [state, setState] = useState<UseApiState<T>>({
    data: null,
    isLoading: false,
    error: null,
  });

  const execute = useCallback(async (apiCall: () => Promise<T>) => {
    setState({ data: null, isLoading: true, error: null });
    try {
      const data = await apiCall();
      setState({ data, isLoading: false, error: null });
      return data;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred';
      setState({ data: null, isLoading: false, error: errorMessage });
      throw err;
    }
  }, []);

  return { ...state, execute };
}

export function useApiMutation<T, P extends unknown[]>() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutate = useCallback(async (apiCall: (...params: P) => Promise<T>, ...params: P) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await apiCall(...params);
      setIsLoading(false);
      return result;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred';
      setError(errorMessage);
      setIsLoading(false);
      throw err;
    }
  }, []);

  return { mutate, isLoading, error };
}

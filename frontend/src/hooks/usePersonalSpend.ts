import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { PersonalSpend } from '../types';

export function usePersonalSpend() {
  const { user, isApproved } = useAuth();
  const userId = user?.id;
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<{
    userId?: string;
    revision: number;
    data: PersonalSpend | null;
    error: boolean;
  }>({ revision: -1, data: null, error: false });
  const refresh = useCallback(() => setRevision((value) => value + 1), []);

  useEffect(() => {
    window.addEventListener('rally:balances-changed', refresh);
    return () => window.removeEventListener('rally:balances-changed', refresh);
  }, [refresh]);

  useEffect(() => {
    if (!isApproved || !userId) return;
    let cancelled = false;
    api.getMySpend().then(
      (data) => { if (!cancelled) setState({ userId, revision, data, error: false }); },
      () => { if (!cancelled) setState({ userId, revision, data: null, error: true }); },
    );
    return () => { cancelled = true; };
  }, [userId, isApproved, revision]);

  const current = isApproved && state.userId === userId && state.revision === revision;
  return {
    data: current ? state.data : null,
    error: current && state.error,
    isLoading: isApproved && !!userId && !current,
    refresh,
  };
}

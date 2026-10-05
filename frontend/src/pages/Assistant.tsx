import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { formatInTimeZone } from 'date-fns-tz';
import AssistantPanel from '../components/assistant/AssistantPanel';
import { api } from '../services/api';
import type { Session } from '../types';

export default function Assistant() {
  const [params] = useSearchParams();
  const sessionId = params.get('session') || undefined;
  const [session, setSession] = useState<Session | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    setSession(null); setError('');
    if (sessionId) api.getSession(sessionId).then((data) => { if (active) setSession(data.session); }).catch(() => { if (active) setError('This session could not be loaded. Choose a session from Home.'); });
    return () => { active = false; };
  }, [sessionId]);
  return <div className="mx-auto max-w-2xl space-y-5">
    <div className="page-heading"><p className="page-kicker">Your club assistant</p><h1 className="page-title">Ask Rally</h1><p className="mt-2 text-sm text-slate-600">Speak or type. Review every expense before saving.</p></div>
    {session && <div className="rounded-xl border border-primary-200 bg-primary-50 px-4 py-3"><p className="text-sm font-semibold">{session.title}</p><p className="mt-1 text-sm text-slate-600">{formatInTimeZone(session.starts_at || session.session_date, 'Australia/Sydney', 'EEEE, d MMM yyyy')}</p></div>}
    {error ? <div role="alert" className="card p-5"><p>{error}</p><Link className="btn-outline mt-3" to="/dashboard">Back to Home</Link></div> : <AssistantPanel key={sessionId || 'club'} sessionId={sessionId} />}
  </div>;
}

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, CalendarDays, Loader2, AlertTriangle } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Club, Session, PastSession } from '../types';
import SessionCard from '../components/sessions/SessionCard';
import { nextScheduledSession } from '../utils/session-display';
import { displayName } from '../utils/members';

export default function Dashboard() {
  const { user, isAdmin } = useAuth();
  const [unsettled, setUnsettled] = useState<PastSession[]>([]);
  const [expenseError, setExpenseError] = useState(false);
  useEffect(() => {
    let active = true;
    const refresh = () => api.listUnsettledSessions().then((data) => {
      if (active) { setUnsettled(data.items); setExpenseError(false); }
    }).catch(() => { if (active) setExpenseError(true); });
    void refresh();
    window.addEventListener('focus', refresh);
    const timer = setInterval(refresh, 60_000);
    return () => { active = false; clearInterval(timer); window.removeEventListener('focus', refresh); };
  }, []);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [cancelledSessions, setCancelledSessions] = useState<Session[]>([]);
  const [club, setClub] = useState<Club | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [sessionsData, cancelledData, clubData] = await Promise.all([
        api.listSessions(),
        api.listCancelledSessions(),
        api.getClub(),
      ]);
      setSessions(sessionsData);
      setCancelledSessions(cancelledData);
      setClub(clubData);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const upcomingSessions = sessions.slice(0, 2);
  const nextSession = upcomingSessions[0];
  const futureSessions = upcomingSessions.slice(1);

  return (
    <div className="space-y-7">
      <div className="page-heading">
        <p className="page-kicker">Welcome back, {displayName(user).split(' ')[0]}</p>
        <h1 className="page-title">Ready for your next game?</h1>
      </div>

      {expenseError && <p role="alert" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">Could not check outstanding expenses. <Link className="underline" to="/sessions">Check session history</Link>.</p>}
      {unsettled.length > 0 && <section aria-labelledby="unsettled-heading" className="rounded-2xl border border-amber-200 bg-amber-50/60 p-4 sm:p-5">
        <h2 id="unsettled-heading" className="font-semibold text-slate-950">Expenses to record <span className="ml-2 rounded-full bg-amber-100 px-2 py-0.5 text-sm">{unsettled.length}</span></h2>
        <p className="mt-1 text-sm text-slate-600">{isAdmin ? 'These sessions have finished. Record their costs to update club balances.' : 'These sessions are waiting for an admin to record their expenses.'}</p>
        <ul className="mt-3 divide-y divide-amber-200/60">{unsettled.map((session) => <li key={session.session_id} className="flex flex-wrap items-center justify-between gap-3 py-3">
          <div><p className="text-sm font-semibold">{session.title}</p><p className="mt-1 text-xs text-slate-600">{session.ends_at && formatInTimeZone(session.ends_at, 'Australia/Sydney', 'EEE, d MMM yyyy')} · {session.player_count} confirmed players</p></div>
          {isAdmin ? <Link className="btn-primary text-sm" to={`/assistant?session=${session.session_id}`}>Record expense</Link> : <span className="text-xs font-medium text-amber-800">Expense pending</span>}
        </li>)}</ul>
      </section>}

      {isLoading ? (
        <div className="card flex min-h-48 items-center justify-center" aria-label="Loading upcoming sessions">
          <Loader2 className="h-8 w-8 animate-spin text-primary-700" />
        </div>
      ) : !nextSession ? (
        <div className="card px-5 py-10 text-center">
          <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-100">
            <CalendarDays className="h-6 w-6 text-slate-400" />
          </span>
          <h2 className="mt-4 font-semibold text-slate-900">No upcoming games</h2>
          <p className="mt-1 text-sm text-slate-500">New sessions will appear here when they are scheduled.</p>
        </div>
      ) : (
        <section aria-labelledby="next-session-heading">
          <div className="mb-3 flex items-center justify-between">
            <h2 id="next-session-heading" className="text-base font-semibold text-slate-950">Your next game</h2>
            <Link to={`/sessions/${nextSession.id}`} className="flex min-h-11 items-center gap-1 px-1 text-sm font-semibold text-primary-700 hover:text-primary-800">
              Details <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
          <SessionCard session={nextSession} venueName={club?.venue_name} courtNumber={club?.court_number} timeFormat={club?.time_format} featured />
        </section>
      )}

      {cancelledSessions.length > 0 && (
        <section aria-labelledby="updates-heading">
          <h2 id="updates-heading" className="mb-3 text-base font-semibold text-slate-950">Important updates</h2>
          <div className="space-y-2">
            {cancelledSessions.map((session) => {
              const next = nextScheduledSession(sessions, session);
              return (
                <div
                  key={session.id}
                  className="flex items-start gap-3 rounded-2xl border border-red-200 bg-red-50 p-4"
                >
                  <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600" />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-red-800">
                      Session Cancelled: {formatInTimeZone(session.starts_at || session.session_date, 'Australia/Sydney', 'EEEE, d MMMM yyyy')}
                    </p>
                    <p className="text-sm text-red-600 mt-0.5 whitespace-pre-line break-words">
                      {session.cancellation_reason || 'No reason provided.'}
                    </p>
                    <p className="mt-1 text-sm text-red-800">
                      {next
                        ? `Next scheduled session: ${formatInTimeZone(next.starts_at || next.session_date, 'Australia/Sydney', 'EEEE, d MMMM yyyy')}`
                        : 'No next session is scheduled.'}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {futureSessions.length > 0 && (
        <section aria-labelledby="future-sessions-heading">
          <div className="mb-3 flex items-center justify-between">
            <h2 id="future-sessions-heading" className="text-base font-semibold text-slate-950">Future sessions</h2>
            <Link to="/sessions" className="flex min-h-11 items-center gap-1 px-1 text-sm font-semibold text-primary-700 hover:text-primary-800">
              View sessions <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
          <div className="space-y-3">
            {futureSessions.map((session) => <SessionCard key={session.id} session={session} venueName={club?.venue_name} courtNumber={club?.court_number} timeFormat={club?.time_format} />)}
          </div>
        </section>
      )}

    </div>
  );
}

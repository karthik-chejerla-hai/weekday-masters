import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, CalendarDays, Loader2, AlertTriangle, WalletCards } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Session } from '../types';
import SessionCard from '../components/sessions/SessionCard';
import { displayName } from '../utils/members';

export default function Dashboard() {
  const { user } = useAuth();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [cancelledSessions, setCancelledSessions] = useState<Session[]>([]);
  const [venueName, setVenueName] = useState<string>('');
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
      setVenueName(clubData.venue_name || '');
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const upcomingSessions = sessions.slice(0, 3);
  const nextSession = upcomingSessions[0];
  const laterSessions = upcomingSessions.slice(1);

  return (
    <div className="space-y-7">
      <div className="page-heading">
        <p className="page-kicker">Welcome back, {displayName(user).split(' ')[0]}</p>
        <h1 className="page-title">Ready for your next game?</h1>
      </div>

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
          <SessionCard session={nextSession} venueName={venueName} featured />
        </section>
      )}

      <nav aria-label="Quick links" className="grid grid-cols-2 gap-3">
        <Link to="/sessions" className="card flex min-h-[76px] items-center gap-3 p-3.5 transition-colors hover:bg-slate-50">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700">
            <CalendarDays className="h-5 w-5" />
          </span>
          <span className="min-w-0">
            <span className="block text-sm font-semibold text-slate-950">All sessions</span>
            <span className="block truncate text-xs text-slate-500">Schedule and history</span>
          </span>
        </Link>
        <Link to="/money" className="card flex min-h-[76px] items-center gap-3 p-3.5 transition-colors hover:bg-slate-50">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700">
            <WalletCards className="h-5 w-5" />
          </span>
          <span className="min-w-0">
            <span className="block text-sm font-semibold text-slate-950">Money</span>
            <span className="block truncate text-xs text-slate-500">Balances and ledger</span>
          </span>
        </Link>
      </nav>

      {cancelledSessions.length > 0 && (
        <section aria-labelledby="updates-heading">
          <h2 id="updates-heading" className="mb-3 text-base font-semibold text-slate-950">Important updates</h2>
          <div className="space-y-2">
            {cancelledSessions.map((session) => (
              <div
                key={session.id}
                className="flex items-start gap-3 rounded-2xl border border-red-200 bg-red-50 p-4"
              >
                <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-red-800">
                    Session Cancelled: {format(parseISO(session.session_date), 'EEEE, d MMMM yyyy')}
                  </p>
                  {session.cancellation_reason && (
                    <p className="text-sm text-red-600 mt-0.5">
                      {session.cancellation_reason}
                    </p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {laterSessions.length > 0 && (
        <section aria-labelledby="later-sessions-heading">
          <div className="mb-3 flex items-center justify-between">
            <h2 id="later-sessions-heading" className="text-base font-semibold text-slate-950">Later sessions</h2>
            <Link to="/sessions" className="flex min-h-11 items-center gap-1 px-1 text-sm font-semibold text-primary-700 hover:text-primary-800">
              See all <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
          <div className="space-y-3">
            {laterSessions.map((session) => <SessionCard key={session.id} session={session} venueName={venueName} />)}
          </div>
        </section>
      )}

    </div>
  );
}

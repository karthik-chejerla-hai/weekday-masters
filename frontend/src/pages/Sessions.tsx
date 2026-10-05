import { useCallback, useEffect, useState } from 'react';
import { CalendarDays, Loader2 } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Club, PastSession, Session } from '../types';
import SessionCard from '../components/sessions/SessionCard';
import PastSessionCard from '../components/sessions/PastSessionCard';

type Tab = 'upcoming' | 'history';

const TABS: Array<{ id: Tab; label: string }> = [
  { id: 'upcoming', label: 'Upcoming' },
  { id: 'history', label: 'History' },
];

export default function Sessions() {
  const { isAdmin } = useAuth();
  const [tab, setTab] = useState<Tab>('upcoming');

  const [sessions, setSessions] = useState<Session[]>([]);
  const [past, setPast] = useState<PastSession[]>([]);
  const [club, setClub] = useState<Club | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const [sessionsData, clubData] = await Promise.all([api.listSessions(), api.getClub()]);
        setSessions(sessionsData.slice(0, 2));
        setClub(clubData);
      } catch (error) {
        console.error('Failed to load data:', error);
      } finally {
        setIsLoading(false);
      }
    })();
  }, []);

  // History is fetched when it is first opened rather than up front — most
  // visits to this page are to RSVP for the next game.
  const loadHistory = useCallback(async () => {
    setIsLoadingHistory(true);
    try {
      const { items } = await api.listSessionHistory();
      setPast(items);
    } catch (error) {
      console.error('Failed to load session history:', error);
    } finally {
      setIsLoadingHistory(false);
    }
  }, []);

  useEffect(() => {
    if (tab === 'history' && past.length === 0 && !isLoadingHistory) {
      loadHistory();
    }
  }, [tab, past.length, isLoadingHistory, loadHistory]);

  return (
    <div className="space-y-6">
      <div className="page-heading">
        <p className="page-kicker">Club schedule</p>
        <h1 className="page-title">Sessions</h1>
        <p className="page-description">RSVP for upcoming games or review sessions you have already played.</p>
      </div>

      <div className="grid grid-cols-2 gap-1 rounded-xl bg-slate-100 p-1" role="tablist" aria-label="Session views">
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`min-h-11 rounded-lg px-4 py-2 text-sm font-semibold transition-colors ${
              tab === id
                ? 'bg-white text-slate-950 shadow-sm'
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'upcoming' && (
        <>
          {isLoading ? (
            <div className="card flex min-h-40 items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary-700" />
            </div>
          ) : sessions.length === 0 ? (
            <div className="card p-8 text-center">
              <CalendarDays className="mx-auto mb-4 h-12 w-12 text-slate-300" />
              <p className="font-medium text-slate-700">No upcoming sessions scheduled</p>
              <p className="text-sm text-slate-500 mt-1">Check back later for new sessions</p>
            </div>
          ) : (
            <div className="space-y-3">
              {sessions.map((session) => (
                <SessionCard key={session.id} session={session} venueName={club?.venue_name} courtNumber={club?.court_number} timeFormat={club?.time_format} />
              ))}
            </div>
          )}
        </>
      )}

      {tab === 'history' && (
        <>
          {isLoadingHistory ? (
            <div className="card flex min-h-40 items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary-700" />
            </div>
          ) : past.length === 0 ? (
            <div className="card p-8 text-center">
              <CalendarDays className="mx-auto mb-4 h-12 w-12 text-slate-300" />
              <p className="font-medium text-slate-700">No sessions have been played yet</p>
            </div>
          ) : (
            <div className="space-y-3">
              {past.map((session) => (
                <PastSessionCard key={session.session_id} session={session} isAdmin={isAdmin} />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

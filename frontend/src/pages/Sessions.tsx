import { useCallback, useEffect, useState } from 'react';
import { Calendar, Loader2 } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { PastSession, Session } from '../types';
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
  const [venueName, setVenueName] = useState<string>('');
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const [sessionsData, clubData] = await Promise.all([api.listSessions(), api.getClub()]);
        setSessions(sessionsData);
        setVenueName(clubData.venue_name || '');
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
    <div>
      <div className="mb-4">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          <Calendar className="w-7 h-7 text-primary-600" />
          Sessions
        </h1>
      </div>

      <div className="flex gap-1 border-b border-slate-200 mb-6" role="tablist">
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === id
                ? 'border-primary-600 text-primary-700'
                : 'border-transparent text-slate-500 hover:text-slate-700'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'upcoming' && (
        <>
          {isLoading ? (
            <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
              <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
            </div>
          ) : sessions.length === 0 ? (
            <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
              <Calendar className="w-12 h-12 text-slate-300 mx-auto mb-4" />
              <p className="text-slate-600">No upcoming sessions scheduled</p>
              <p className="text-sm text-slate-500 mt-1">Check back later for new sessions</p>
            </div>
          ) : (
            <div className="space-y-4">
              {sessions.map((session) => (
                <SessionCard key={session.id} session={session} venueName={venueName} />
              ))}
            </div>
          )}
        </>
      )}

      {tab === 'history' && (
        <>
          {isLoadingHistory ? (
            <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
              <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
            </div>
          ) : past.length === 0 ? (
            <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
              <Calendar className="w-12 h-12 text-slate-300 mx-auto mb-4" />
              <p className="text-slate-600">No sessions have been played yet</p>
            </div>
          ) : (
            <div className="space-y-4">
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

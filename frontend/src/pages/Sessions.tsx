import { useEffect, useState } from 'react';
import { Calendar, Loader2 } from 'lucide-react';
import { api } from '../services/api';
import type { Session } from '../types';
import SessionCard from '../components/sessions/SessionCard';

export default function Sessions() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [venueName, setVenueName] = useState<string>('');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [sessionsData, clubData] = await Promise.all([
        api.listSessions(),
        api.getClub(),
      ]);
      setSessions(sessionsData);
      setVenueName(clubData.venue_name || '');
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          <Calendar className="w-7 h-7 text-primary-600" />
          Sessions
        </h1>
        <p className="text-slate-600 mt-1">
          View and RSVP for upcoming badminton sessions
        </p>
      </div>

      {isLoading ? (
        <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
          <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
        </div>
      ) : sessions.length === 0 ? (
        <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
          <Calendar className="w-12 h-12 text-slate-300 mx-auto mb-4" />
          <p className="text-slate-600">No upcoming sessions scheduled</p>
          <p className="text-sm text-slate-500 mt-1">
            Check back later for new sessions
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {sessions.map((session) => (
            <SessionCard key={session.id} session={session} venueName={venueName} />
          ))}
        </div>
      )}
    </div>
  );
}

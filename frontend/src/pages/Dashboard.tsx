import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Calendar, ArrowRight, Loader2, AlertTriangle } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { useAuth } from '../context/AuthContext';
import { api } from '../services/api';
import type { Session } from '../types';
import SessionCard from '../components/sessions/SessionCard';

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
  const getConfirmedCount = (session: Session) =>
    session.rsvps?.filter(r => r.status === 'in').length || 0;

  return (
    <div className="space-y-6">
      <div className="bg-gradient-to-r from-primary-600 to-primary-700 rounded-xl p-6 text-white">
        <h1 className="text-2xl font-bold mb-2">
          Welcome back, {user?.name?.split(' ')[0]}!
        </h1>
        <p className="text-primary-100">
          Ready for some badminton? Check out the upcoming sessions below.
        </p>
      </div>

      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-slate-900 flex items-center gap-2">
            <Calendar className="w-5 h-5 text-primary-600" />
            Upcoming Sessions
          </h2>
          <Link
            to="/sessions"
            className="text-primary-600 hover:text-primary-700 text-sm font-medium flex items-center gap-1"
          >
            View All
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>

        {/* Cancelled Session Banners */}
        {cancelledSessions.length > 0 && (
          <div className="space-y-2 mb-4">
            {cancelledSessions.map((session) => (
              <div
                key={session.id}
                className="bg-red-50 border border-red-200 rounded-lg p-3 flex items-start gap-3"
              >
                <AlertTriangle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
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
        )}

        {isLoading ? (
          <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
            <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
          </div>
        ) : upcomingSessions.length === 0 ? (
          <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
            <Calendar className="w-12 h-12 text-slate-300 mx-auto mb-4" />
            <p className="text-slate-600">No upcoming sessions scheduled</p>
          </div>
        ) : (
          <div className="space-y-4">
            {upcomingSessions.map((session) => (
              <SessionCard key={session.id} session={session} venueName={venueName} />
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="bg-white rounded-xl border border-slate-200 p-4 text-center">
          <p className="text-3xl font-bold text-primary-600">{sessions.length}</p>
          <p className="text-sm text-slate-600">Upcoming Sessions</p>
        </div>
        <div className="bg-white rounded-xl border border-slate-200 p-4 text-center">
          <p className="text-3xl font-bold text-secondary-500">
            {sessions.reduce((acc, s) => acc + getConfirmedCount(s), 0)}
          </p>
          <p className="text-sm text-slate-600">Confirmed RSVPs</p>
        </div>
      </div>
    </div>
  );
}

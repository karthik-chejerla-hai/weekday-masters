import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Calendar, Clock, Users, MapPin, AlertCircle, Loader2 } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { useAuth } from '../context/AuthContext';
import { api } from '../services/api';
import type { Session, RSVPSummary, RSVPStatus, RSVP } from '../types';
import RSVPButton from '../components/rsvp/RSVPButton';
import PlayerList from '../components/rsvp/PlayerList';
import Badge from '../components/ui/Badge';

export default function SessionDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();

  const [session, setSession] = useState<Session | null>(null);
  const [summary, setSummary] = useState<RSVPSummary | null>(null);
  const [myRsvp, setMyRsvp] = useState<RSVP | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (id) loadSession();
  }, [id]);

  const loadSession = async () => {
    if (!id) return;
    try {
      const data = await api.getSession(id);
      setSession(data.session);
      setSummary(data.rsvp_summary);
      const userRsvp = data.session.rsvps?.find(r => r.user_id === user?.id);
      setMyRsvp(userRsvp || null);
    } catch (error) {
      console.error('Failed to load session:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleRSVP = async (status: RSVPStatus) => {
    if (!id) return;
    await api.createRSVP(id, status);
    await loadSession();
  };

  if (isLoading) {
    return (
      <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
      </div>
    );
  }

  if (!session) {
    return (
      <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
        <AlertCircle className="w-12 h-12 text-red-400 mx-auto mb-4" />
        <p className="text-slate-600">Session not found</p>
        <button onClick={() => navigate('/sessions')} className="btn-primary mt-4">
          Back to Sessions
        </button>
      </div>
    );
  }

  const sessionDate = parseISO(session.session_date);
  const isDeadlinePassed = new Date() > new Date(session.rsvp_deadline);
  const isCancelled = session.status === 'cancelled';
  const canChangeRsvp = !isDeadlinePassed && !isCancelled;
  const canOnlyChangeToIn = isDeadlinePassed && myRsvp?.status === 'in';

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate(-1)}
        className="flex items-center gap-2 text-slate-600 hover:text-slate-900"
      >
        <ArrowLeft className="w-5 h-5" />
        Back
      </button>

      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <div className="flex items-start justify-between mb-4">
          <h1 className="text-2xl font-bold text-slate-900">{session.title}</h1>
          {isCancelled ? (
            <Badge variant="danger">Cancelled</Badge>
          ) : isDeadlinePassed ? (
            <Badge variant="warning">RSVP Closed</Badge>
          ) : (
            <Badge variant="success">Open for RSVP</Badge>
          )}
        </div>

        {session.description && (
          <p className="text-slate-600 mb-4">{session.description}</p>
        )}

        <div className="grid sm:grid-cols-2 gap-4 text-sm">
          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <Calendar className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">{format(sessionDate, 'EEEE, d MMMM yyyy')}</p>
              <p className="text-slate-500">Date</p>
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <Clock className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">{session.start_time} - {session.end_time}</p>
              <p className="text-slate-500">Time</p>
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <Users className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">
                {summary?.total_in || 0} / {session.max_players} players
              </p>
              <p className="text-slate-500">{session.courts} court{session.courts > 1 ? 's' : ''}</p>
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <MapPin className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">Club Venue</p>
              <p className="text-slate-500">Location</p>
            </div>
          </div>
        </div>

        {!isDeadlinePassed && (
          <div className="mt-4 p-3 bg-amber-50 rounded-lg">
            <p className="text-sm text-amber-800">
              <span className="font-medium">RSVP Deadline:</span>{' '}
              {format(new Date(session.rsvp_deadline), "EEEE, d MMMM yyyy 'at' h:mm a")}
            </p>
          </div>
        )}
      </div>

      {!isCancelled && (
        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <h2 className="font-semibold text-slate-900 mb-4">Your RSVP</h2>

          {canOnlyChangeToIn ? (
            <div className="p-4 bg-amber-50 rounded-lg text-amber-800 text-sm">
              <p className="font-medium">RSVP deadline has passed</p>
              <p>You confirmed attendance and cannot change your RSVP.</p>
            </div>
          ) : (
            <>
              <RSVPButton
                currentStatus={myRsvp?.status}
                onRSVP={handleRSVP}
                disabled={!canChangeRsvp}
              />
              {!canChangeRsvp && !canOnlyChangeToIn && (
                <p className="text-sm text-slate-500 mt-2 text-center">
                  RSVP is closed for this session
                </p>
              )}
            </>
          )}
        </div>
      )}

      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <PlayerList
          rsvps={session.rsvps || []}
          maxPlayers={session.max_players}
        />
      </div>
    </div>
  );
}

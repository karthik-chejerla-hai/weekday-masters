import { useEffect, useRef, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Calendar, Clock, Users, MapPin, AlertCircle, Loader2 } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { formatInTimeZone, fromZonedTime } from 'date-fns-tz';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Club, Session, RSVPSummary, SelectableRSVPStatus, RSVP, GuestRSVP } from '../types';
import { formatSessionTimeRange, formatSessionVenue } from '../utils/session-display';
import { sessionResponses } from '../utils/player-responses';
import RSVPButton from '../components/rsvp/RSVPButton';
import PlayerList from '../components/rsvp/PlayerList';
import AdminRSVPForm from '../components/rsvp/AdminRSVPForm';
import Badge from '../components/ui/Badge';

export default function SessionDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();

  const [session, setSession] = useState<Session | null>(null);
  const [club, setClub] = useState<Club | null>(null);
  const [summary, setSummary] = useState<RSVPSummary | null>(null);
  const [myRsvp, setMyRsvp] = useState<RSVP | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [editingGuest, setEditingGuest] = useState<GuestRSVP | null>(null);
  const guestDialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    if (editingGuest) guestDialog.current?.showModal();
    else if (guestDialog.current?.open) guestDialog.current.close();
  }, [editingGuest]);

  useEffect(() => {
    if (id) loadSession();
  }, [id]);

  const loadSession = async () => {
    if (!id) return;
    try {
      const [data, clubData] = await Promise.all([api.getSession(id), api.getClub().catch(() => null)]);
      setSession(data.session);
      setClub(clubData);
      setSummary(data.rsvp_summary);
      const userRsvp = data.session.rsvps?.find(r => r.user_id === user?.id);
      setMyRsvp(userRsvp || null);
    } catch (error) {
      console.error('Failed to load session:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleRSVP = async (status: SelectableRSVPStatus) => {
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
  const startsAt = session.starts_at ? Date.parse(session.starts_at)
    : fromZonedTime(`${session.session_date.slice(0, 10)}T${session.start_time}:00`, 'Australia/Sydney').getTime();
  const emphasizeResults = !isCancelled && startsAt <= Date.now();
  const responseLabel = myRsvp ? {
    in: 'Confirmed', maybe: 'Tentative', out: 'Not attending', waitlisted: 'Waitlisted',
  }[myRsvp.status] : 'Not responded';
  const canChangeRsvp = !isDeadlinePassed && session.status === 'open';
  const canOnlyChangeToIn = isDeadlinePassed && myRsvp?.status === 'in';
  const isWaitlisted = myRsvp?.status === 'waitlisted';
  const confirmedCount = summary?.total_in ?? 0;
  // The backend is the authority on capacity; spots_left already accounts for
  // admin-added players pushing the confirmed count past max_players.
  const isFull = summary ? summary.spots_left <= 0 : false;

  const capacityNotice = isWaitlisted
    ? {
        title: `You're on the waitlist${myRsvp?.waitlist_position ? ` at position #${myRsvp.waitlist_position}` : ''}`,
        body: "This session is full. We'll confirm you automatically — and notify you — as soon as a spot opens up.",
      }
    : isFull && myRsvp?.status !== 'in'
      ? {
          title: 'This session is full',
          body: "You can still join the waitlist and you'll be confirmed if someone drops out.",
        }
      : null;

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
          ) : isDeadlinePassed || session.status === 'closed' ? (
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
              <p className="font-medium">{formatSessionTimeRange(session.start_time, session.end_time, club?.time_format)}</p>
              <p className="text-slate-500">Time</p>
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <Users className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">
                {confirmedCount} / {session.max_players} players
                {!!summary?.total_waitlisted && (
                  <span className="text-amber-600 font-normal"> · {summary.total_waitlisted} waiting</span>
                )}
              </p>
              <p className="text-slate-500">{session.courts} court{session.courts > 1 ? 's' : ''}</p>
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-700">
            <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
              <MapPin className="w-5 h-5 text-primary-600" />
            </div>
            <div>
              <p className="font-medium">{formatSessionVenue(club?.venue_name, club?.court_number)}</p>
              <p className="text-slate-500">Location</p>
            </div>
          </div>
        </div>

        {isCancelled && (
          <p role="status" className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-800 whitespace-pre-line break-words">
            {session.cancellation_reason || 'No reason provided.'}
          </p>
        )}

        {canChangeRsvp && (
          <div className="mt-4 p-3 bg-amber-50 rounded-lg">
            <p className="text-sm text-amber-800">
              <span className="font-medium">RSVP Deadline:</span>{' '}
              {formatInTimeZone(session.rsvp_deadline, 'Australia/Sydney', `EEEE, d MMMM yyyy 'at' ${club?.time_format === '12h' ? 'h:mm a' : 'HH:mm'}`)}
            </p>
          </div>
        )}
      </div>

      {!isCancelled && (
        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <h2 className="font-semibold text-slate-900">Your RSVP</h2>
          <p className="mt-1 mb-4 text-sm font-medium text-slate-700">Current response: {responseLabel}</p>

          {canOnlyChangeToIn ? (
            <div className="p-4 bg-amber-50 rounded-lg text-amber-800 text-sm">
              <p className="font-medium">RSVP deadline has passed</p>
              <p>You confirmed attendance and cannot change your RSVP.</p>
            </div>
          ) : (
            <>
              {capacityNotice && (
                <div className="mb-3 p-3 bg-amber-50 rounded-lg text-amber-800 text-sm">
                  <p className="font-medium">{capacityNotice.title}</p>
                  <p>{capacityNotice.body}</p>
                </div>
              )}

              <RSVPButton
                currentStatus={myRsvp?.status}
                onRSVP={handleRSVP}
                disabled={!canChangeRsvp}
                isFull={isFull}
                waitlistPosition={myRsvp?.waitlist_position}
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

      {user?.role === 'admin' && session.status === 'open' && (
        <AdminRSVPForm key={session.id} sessionId={session.id} rsvps={session.rsvps || []}
          guests={session.guest_rsvps || []} allowGuests={startsAt > Date.now()}
          deadlinePassed={isDeadlinePassed} isFull={isFull} onSaved={loadSession} />
      )}

      <Link
        to={`/sessions/${id}/games`}
        className={emphasizeResults
          ? 'btn-primary w-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-700 focus-visible:ring-offset-2'
          : 'inline-flex min-h-11 items-center rounded-lg px-2 text-sm font-semibold text-primary-800 underline underline-offset-4 hover:text-primary-950 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-700 focus-visible:ring-offset-2'}
      >Game scores and results</Link>

      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <PlayerList
          rsvps={sessionResponses(session)}
          maxPlayers={session.max_players}
          onEditGuest={user?.role === 'admin' && session.status === 'open' && startsAt > Date.now() ? setEditingGuest : undefined}
        />
      </div>
      <dialog ref={guestDialog} aria-label="Edit guest RSVP" onClose={() => setEditingGuest(null)}
        className="m-auto w-[calc(100%-2rem)] max-w-lg max-h-[90dvh] overflow-y-auto rounded-xl p-0 backdrop:bg-slate-900/40">
        {editingGuest && <AdminRSVPForm key={editingGuest.id} sessionId={session.id} rsvps={session.rsvps || []}
          guests={session.guest_rsvps || []} initialGuest={editingGuest}
          deadlinePassed={isDeadlinePassed} isFull={isFull}
          onCancelEdit={() => setEditingGuest(null)} onSaved={async () => {
            await loadSession();
            setEditingGuest(null);
          }} />}
      </dialog>
    </div>
  );
}

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Plus, Calendar, Trash2, Loader2, XCircle, X } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { api } from '../services/api';
import type { Session, CreateSessionInput } from '../types';
import Badge from '../components/ui/Badge';

export default function AdminSessions() {
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Cancel modal state
  const [cancellingSession, setCancellingSession] = useState<Session | null>(null);
  const [cancelReason, setCancelReason] = useState('');
  const [isCancelling, setIsCancelling] = useState(false);

  const [formData, setFormData] = useState<CreateSessionInput>({
    title: '',
    description: '',
    session_date: '',
    start_time: '20:00',
    end_time: '22:00',
    courts: 2,
    is_recurring: false,
    occurrences: 4,
  });

  // Auto-generate title based on date
  const generateTitle = (dateStr: string) => {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return format(date, 'EEEE - dd MMM yyyy');
  };

  useEffect(() => {
    loadSessions();
  }, []);

  const loadSessions = async () => {
    try {
      const data = await api.listSessions();
      setSessions(data);
    } catch (error) {
      console.error('Failed to load sessions:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    try {
      const input: CreateSessionInput = {
        ...formData,
        title: generateTitle(formData.session_date),
        recurring_day_of_week: formData.is_recurring
          ? new Date(formData.session_date).getDay()
          : undefined,
        occurrences: formData.is_recurring ? formData.occurrences : undefined,
      };
      await api.createSession(input);
      setShowForm(false);
      setFormData({
        title: '',
        description: '',
        session_date: '',
        start_time: '20:00',
        end_time: '22:00',
        courts: 2,
        is_recurring: false,
        occurrences: 4,
      });
      await loadSessions();
    } catch (error) {
      console.error('Failed to create session:', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this session?')) return;
    setDeletingId(id);
    try {
      await api.deleteSession(id);
      setSessions(prev => prev.filter(s => s.id !== id));
    } catch (error) {
      console.error('Failed to delete session:', error);
    } finally {
      setDeletingId(null);
    }
  };

  const handleCancelSession = async () => {
    if (!cancellingSession) return;
    setIsCancelling(true);
    try {
      const updated = await api.cancelSession(cancellingSession.id, cancelReason);
      setSessions(prev => prev.map(s => s.id === updated.id ? updated : s));
      setCancellingSession(null);
      setCancelReason('');
    } catch (error) {
      console.error('Failed to cancel session:', error);
    } finally {
      setIsCancelling(false);
    }
  };

  const getRsvpCount = (session: Session) =>
    session.rsvps?.filter(r => r.status === 'in').length || 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <button
          onClick={() => navigate('/admin')}
          className="flex items-center gap-2 text-slate-600 hover:text-slate-900"
        >
          <ArrowLeft className="w-5 h-5" />
          Back
        </button>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-primary-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors flex items-center gap-2"
        >
          <Plus className="w-5 h-5" />
          New Session
        </button>
      </div>

      <div>
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          <Calendar className="w-7 h-7 text-primary-600" />
          Manage Sessions
        </h1>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
          <h2 className="font-semibold text-slate-900 mb-4">Create New Session</h2>

          {/* Session Type Radio Buttons */}
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-2">Session Type *</label>
            <div className="flex gap-6">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="session_type"
                  checked={!formData.is_recurring}
                  onChange={() => setFormData({ ...formData, is_recurring: false })}
                  className="w-4 h-4 text-primary-600"
                />
                <span className="text-sm text-slate-700">One-off Session</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="session_type"
                  checked={formData.is_recurring}
                  onChange={() => setFormData({ ...formData, is_recurring: true })}
                  className="w-4 h-4 text-primary-600"
                />
                <span className="text-sm text-slate-700">Recurring Weekly</span>
              </label>
            </div>
          </div>

          <div className="grid sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Date *</label>
              <input
                type="date"
                required
                value={formData.session_date}
                onChange={(e) => setFormData({ ...formData, session_date: e.target.value })}
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>

            {formData.is_recurring && (
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Number of Occurrences *</label>
                <select
                  value={formData.occurrences}
                  onChange={(e) => setFormData({ ...formData, occurrences: parseInt(e.target.value) })}
                  className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
                >
                  {[2, 3, 4, 5, 6, 7, 8, 10, 12].map((n) => (
                    <option key={n} value={n}>{n} weeks</option>
                  ))}
                </select>
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Start Time *</label>
              <input
                type="time"
                required
                value={formData.start_time}
                onChange={(e) => setFormData({ ...formData, start_time: e.target.value })}
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">End Time *</label>
              <input
                type="time"
                required
                value={formData.end_time}
                onChange={(e) => setFormData({ ...formData, end_time: e.target.value })}
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Courts *</label>
              <select
                value={formData.courts}
                onChange={(e) => setFormData({ ...formData, courts: parseInt(e.target.value) })}
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
              >
                <option value={1}>1 Court (Max 6 players)</option>
                <option value={2}>2 Courts (Max 10 players)</option>
                <option value={3}>3 Courts (Max 16 players)</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Description</label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
              rows={2}
              placeholder="Optional description..."
            />
          </div>

          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={isSubmitting}
              className="bg-primary-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {isSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
              Create Session
            </button>
            <button
              type="button"
              onClick={() => setShowForm(false)}
              className="px-6 py-2 rounded-lg font-medium text-slate-600 hover:bg-slate-100 transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {isLoading ? (
        <div className="bg-white rounded-xl border border-slate-200 p-8 flex items-center justify-center">
          <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
        </div>
      ) : sessions.length === 0 ? (
        <div className="bg-white rounded-xl border border-slate-200 p-8 text-center">
          <Calendar className="w-12 h-12 text-slate-300 mx-auto mb-4" />
          <p className="text-slate-600">No sessions created yet</p>
          <p className="text-sm text-slate-500 mt-1">Click "New Session" to create one</p>
        </div>
      ) : (
        <div className="space-y-3">
          {sessions.map((session) => (
            <div
              key={session.id}
              className="bg-white rounded-xl border border-slate-200 p-4 flex items-center justify-between"
            >
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="font-medium text-slate-900">{session.title}</h3>
                  {session.status === 'cancelled' && <Badge variant="danger">Cancelled</Badge>}
                  {session.is_recurring && <Badge variant="info">Recurring</Badge>}
                </div>
                <p className="text-sm text-slate-600">
                  {format(parseISO(session.session_date), 'EEE, d MMM yyyy')} | {session.start_time} - {session.end_time}
                </p>
                <p className="text-sm text-slate-500">
                  {getRsvpCount(session)} / {session.max_players} players | {session.courts} court{session.courts > 1 ? 's' : ''}
                </p>
                {session.status === 'cancelled' && session.cancellation_reason && (
                  <p className="text-sm text-red-600 mt-1">
                    Reason: {session.cancellation_reason}
                  </p>
                )}
              </div>
              <div className="flex items-center gap-2">
                {session.status !== 'cancelled' && (
                  <button
                    onClick={() => setCancellingSession(session)}
                    className="p-2 text-amber-600 hover:bg-amber-50 rounded-lg transition-colors"
                    title="Cancel Session"
                  >
                    <XCircle className="w-5 h-5" />
                  </button>
                )}
                <button
                  onClick={() => handleDelete(session.id)}
                  disabled={deletingId === session.id}
                  className="p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors disabled:opacity-50"
                  title="Delete"
                >
                  {deletingId === session.id ? (
                    <Loader2 className="w-5 h-5 animate-spin" />
                  ) : (
                    <Trash2 className="w-5 h-5" />
                  )}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Cancel Session Modal */}
      {cancellingSession && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-xl max-w-md w-full p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-slate-900">Cancel Session</h3>
              <button
                onClick={() => {
                  setCancellingSession(null);
                  setCancelReason('');
                }}
                className="p-1 text-slate-400 hover:text-slate-600"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-slate-600 mb-4">
              Are you sure you want to cancel "{cancellingSession.title}" on{' '}
              {format(parseISO(cancellingSession.session_date), 'EEE, d MMM yyyy')}?
            </p>

            <div className="mb-4">
              <label className="block text-sm font-medium text-slate-700 mb-1">
                Reason for cancellation (optional)
              </label>
              <textarea
                value={cancelReason}
                onChange={(e) => setCancelReason(e.target.value)}
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500"
                rows={3}
                placeholder="e.g., Court unavailable, Not enough players..."
              />
            </div>

            <div className="flex gap-3">
              <button
                onClick={handleCancelSession}
                disabled={isCancelling}
                className="flex-1 bg-red-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-red-700 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {isCancelling && <Loader2 className="w-4 h-4 animate-spin" />}
                Cancel Session
              </button>
              <button
                onClick={() => {
                  setCancellingSession(null);
                  setCancelReason('');
                }}
                className="flex-1 px-4 py-2 rounded-lg font-medium text-slate-600 hover:bg-slate-100 transition-colors"
              >
                Keep Session
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useEffect, useState, useRef } from 'react';
import { Link } from 'react-router-dom';
import { Users, Calendar, Check, X, Loader2, MapPin, Save, Building, Megaphone, Send } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../context/useAuth';
import { announcementRequest, clearAnnouncementRequest } from '../services/announcement-request';
import type { TimeFormat, User } from '../types';
import Avatar from '../components/ui/Avatar';
import { displayName } from '../utils/members';

export default function Admin() {
  const { user } = useAuth();
  const announcementAttempt = useRef<ReturnType<typeof announcementRequest> | null>(null);
  const publishingAnnouncement = useRef(false);
  const [joinRequests, setJoinRequests] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [processingId, setProcessingId] = useState<string | null>(null);

  // Club settings state
  const [clubForm, setClubForm] = useState({ name: '', venue_name: '', venue_address: '', court_number: '', time_format: '24h' as TimeFormat, rsvp_deadline_hours: '50' });
  const [savedDeadlineHours, setSavedDeadlineHours] = useState(50);
  const [isSavingClub, setIsSavingClub] = useState(false);
  const [notificationsPaused, setNotificationsPaused] = useState(false);
  const [clubMessage, setClubMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Announcement state
  const [announcementForm, setAnnouncementForm] = useState({ title: '', body: '' });
  const [isSendingAnnouncement, setIsSendingAnnouncement] = useState(false);
  const [announcementMessage, setAnnouncementMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [requestsData, clubData] = await Promise.all([
        api.listJoinRequests(),
        api.getClub(),
      ]);
      setJoinRequests(requestsData);
      setNotificationsPaused(clubData.notifications_paused ?? false);
      setSavedDeadlineHours(clubData.rsvp_deadline_hours ?? 50);
      setClubForm({
        name: clubData.name || '',
        venue_name: clubData.venue_name || '',
        venue_address: clubData.venue_address || '',
        court_number: clubData.court_number ? String(clubData.court_number) : '',
        time_format: clubData.time_format || '24h',
        rsvp_deadline_hours: String(clubData.rsvp_deadline_hours ?? 50),
      });
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleApprove = async (userId: string) => {
    setProcessingId(userId);
    try {
      await api.approveJoinRequest(userId);
      setJoinRequests(prev => prev.filter(u => u.id !== userId));
    } catch (error) {
      console.error('Failed to approve:', error);
    } finally {
      setProcessingId(null);
    }
  };

  const handleReject = async (userId: string) => {
    setProcessingId(userId);
    try {
      await api.rejectJoinRequest(userId);
      setJoinRequests(prev => prev.filter(u => u.id !== userId));
    } catch (error) {
      console.error('Failed to reject:', error);
    } finally {
      setProcessingId(null);
    }
  };

  const handleSaveClub = async () => {
    const courtNumber = clubForm.court_number === '' ? 0 : Number(clubForm.court_number);
    if (!Number.isInteger(courtNumber) || (clubForm.court_number !== '' && courtNumber < 1) || courtNumber > 999) {
      setClubMessage({ type: 'error', text: 'Enter a whole court number from 1 to 999, or leave it blank.' });
      return;
    }
    const { rsvp_deadline_hours: deadlineValue, ...settings } = clubForm;
    const deadlineHours = Number(deadlineValue);
    if (deadlineValue.trim() === '' || !Number.isInteger(deadlineHours) || deadlineHours < 0 || deadlineHours > 8760) {
      setClubMessage({ type: 'error', text: 'Enter whole hours from 0 to 8760 for the RSVP deadline.' });
      return;
    }
    setIsSavingClub(true);
    setClubMessage(null);
    try {
      const saved = await api.updateClub({
        ...settings,
        court_number: courtNumber,
        ...(deadlineHours !== savedDeadlineHours ? { rsvp_deadline_hours: deadlineHours } : {}),
      });
      const currentDeadlineHours = saved.rsvp_deadline_hours ?? deadlineHours;
      setSavedDeadlineHours(currentDeadlineHours);
      setClubForm(current => ({ ...current, rsvp_deadline_hours: String(currentDeadlineHours) }));
      setClubMessage({ type: 'success', text: 'Club settings saved!' });
    } catch (error) {
      console.error('Failed to save club:', error);
      setClubMessage({ type: 'error', text: 'Failed to save club settings' });
    } finally {
      setIsSavingClub(false);
    }
  };

  const handleSendAnnouncement = async () => {
    if (!announcementForm.title.trim() || !announcementForm.body.trim()) {
      setAnnouncementMessage({ type: 'error', text: 'Please enter both title and message' });
      return;
    }

    if(publishingAnnouncement.current)return;
    publishingAnnouncement.current=true;
    setIsSendingAnnouncement(true);setAnnouncementMessage(null);
    const actor=user?.id || 'admin';
    announcementAttempt.current=announcementRequest(actor,announcementForm.title,announcementForm.body,announcementAttempt.current);
    try {
      const result=await api.sendAnnouncement(announcementForm.title.trim(),announcementForm.body.trim(),announcementAttempt.current.requestId);
      setAnnouncementMessage({type:'success',text:`Announcement saved for ${result.audience_count} members. Push queued for ${result.queued_count}; ${result.suppressed_count} will not receive push.`});
      clearAnnouncementRequest(actor);announcementAttempt.current=null;setAnnouncementForm({title:'',body:''});
    } catch(error) {
      const message=(error as {response?:{data?:{message?:string}}})?.response?.data?.message;
      setAnnouncementMessage({type:'error',text:message || 'Could not confirm publication. Retry without changing the message to check it safely.'});
    } finally {publishingAnnouncement.current=false;setIsSendingAnnouncement(false);}
  };

  return (
    <div className="space-y-6">
      {notificationsPaused && (
        <div role="status" className="rounded-lg border border-secondary-300 bg-secondary-50 p-3 text-sm text-secondary-900">
          Push delivery is paused. Published announcements remain available in notification history.
        </div>
      )}
      <div className="page-heading">
        <p className="page-kicker">Club management</p>
        <h1 className="page-title">Admin</h1>
        <p className="page-description">Review join requests and manage your club.</p>
      </div>

      {/* Join Requests */}
      <div className="card p-4 sm:p-6" aria-labelledby="join-requests-heading">
        <h2 id="join-requests-heading" className="font-semibold text-slate-900 mb-4 flex items-center gap-2">
          <Users className="w-5 h-5 text-primary-600" />
          Join Requests
          <span className="text-sm font-normal text-slate-500">
            ({joinRequests.length} pending)
          </span>
        </h2>

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
          </div>
        ) : joinRequests.length === 0 ? (
          <div className="text-sm text-slate-500">
            <p>No pending join requests</p>
          </div>
        ) : (
          <div className="space-y-3">
            {joinRequests.map((user) => (
              <div
                key={user.id}
                className="flex flex-wrap items-center justify-between gap-3 p-3 bg-slate-50 rounded-lg"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <Avatar src={user.profile_picture} name={displayName(user)} />
                  <div className="min-w-0">
                    <p className="font-medium text-slate-900">{displayName(user)}</p>
                    <p className="break-all text-sm text-slate-500">{user.email}</p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleApprove(user.id)}
                    disabled={processingId === user.id}
                    className="min-h-11 min-w-11 flex items-center justify-center p-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 transition-colors disabled:opacity-50"
                    aria-label={`Approve ${displayName(user)}`}
                  >
                    {processingId === user.id ? (
                      <Loader2 className="w-5 h-5 animate-spin" />
                    ) : (
                      <Check className="w-5 h-5" />
                    )}
                  </button>
                  <button
                    onClick={() => handleReject(user.id)}
                    disabled={processingId === user.id}
                    className="min-h-11 min-w-11 flex items-center justify-center p-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 transition-colors disabled:opacity-50"
                    aria-label={`Reject ${displayName(user)}`}
                  >
                    <X className="w-5 h-5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <nav aria-label="Club management" className="flex flex-wrap gap-3">
        <Link to="/admin/sessions" className="btn-outline gap-2"><Calendar className="h-4 w-4" aria-hidden="true" />Manage sessions</Link>
        <Link to="/admin/members" className="btn-outline gap-2"><Users className="h-4 w-4" aria-hidden="true" />Manage members</Link>
      </nav>

      {/* Club Settings */}
      <section className="card p-4 sm:p-6" aria-labelledby="club-settings-heading">
        <h2 id="club-settings-heading" className="flex items-center gap-2 font-semibold text-slate-900"><Building className="h-5 w-5 text-primary-700" aria-hidden="true" />Club settings</h2>
        <p className="mt-2 text-sm text-slate-600">{isLoading ? 'Loading club settings…' : [clubForm.name, clubForm.venue_name].filter(Boolean).join(' · ')}</p>
        <details className="mt-3">
          <summary className="min-h-11 cursor-pointer py-3 text-sm font-semibold text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600">Edit club settings</summary>
        <div className="mt-3 space-y-4">
          <div>
            <label htmlFor="club-name" className="block text-sm font-medium text-slate-700 mb-1">
              Club Name
            </label>
            <input
              id="club-name"
              type="text"
              value={clubForm.name}
              onChange={(e) => setClubForm({ ...clubForm, name: e.target.value })}
              placeholder="Enter club name"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div>
            <label htmlFor="club-venue-name" className="block text-sm font-medium text-slate-700 mb-1">
              <MapPin className="w-4 h-4 inline mr-1" />
              Venue Name
            </label>
            <input
              id="club-venue-name"
              type="text"
              value={clubForm.venue_name}
              onChange={(e) => setClubForm({ ...clubForm, venue_name: e.target.value })}
              placeholder="e.g., Sydney Olympic Park Sports Centre"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div>
            <label htmlFor="club-venue-address" className="block text-sm font-medium text-slate-700 mb-1">
              Venue Address
            </label>
            <textarea
              id="club-venue-address"
              value={clubForm.venue_address}
              onChange={(e) => setClubForm({ ...clubForm, venue_address: e.target.value })}
              placeholder="Enter full venue address"
              rows={2}
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="club-court-number" className="mb-1 block text-sm font-medium text-slate-700">Court number</label>
              <input
                id="club-court-number"
                type="number"
                min="1"
                max="999"
                step="1"
                value={clubForm.court_number}
                onChange={(e) => setClubForm({ ...clubForm, court_number: e.target.value })}
                placeholder="e.g., 8"
                aria-describedby="club-court-help"
                className="w-full rounded-lg border border-slate-300 px-4 py-2 focus:outline-none focus:ring-2 focus:ring-primary-500"
              />
              <p id="club-court-help" className="mt-1 text-xs text-slate-500">Usual court at this venue. Leave blank if not assigned.</p>
            </div>
            <div>
              <label htmlFor="club-time-format" className="mb-1 block text-sm font-medium text-slate-700">Time display</label>
              <select
                id="club-time-format"
                value={clubForm.time_format}
                onChange={(e) => setClubForm({ ...clubForm, time_format: e.target.value as TimeFormat })}
                aria-describedby="club-time-help"
                className="w-full rounded-lg border border-slate-300 bg-white px-4 py-2 focus:outline-none focus:ring-2 focus:ring-primary-500"
              >
                <option value="12h">12-hour (8:00 PM)</option>
                <option value="24h">24-hour (20:00)</option>
              </select>
              <p id="club-time-help" className="mt-1 text-xs text-slate-500">Used for session times across the app.</p>
            </div>
          </div>

          <div>
            <label htmlFor="club-rsvp-hours" className="mb-1 block text-sm font-medium text-slate-700">Default RSVP deadline (hours before start)</label>
            <input id="club-rsvp-hours" type="number" min="0" max="8760" step="1"
              value={clubForm.rsvp_deadline_hours}
              onChange={(e) => setClubForm({ ...clubForm, rsvp_deadline_hours: e.target.value })}
              aria-describedby="club-rsvp-help" disabled={isLoading || isSavingClub}
              className="w-full rounded-lg border border-slate-300 px-4 py-2 focus:outline-none focus:ring-2 focus:ring-primary-500" />
            <p id="club-rsvp-help" className="mt-2 text-sm text-slate-600">
              Changing this value replaces the RSVP deadlines for all future sessions, including custom deadlines.
              Past sessions stay unchanged. Use 0 to close RSVPs when a session starts.
            </p>
          </div>



          <button
            onClick={handleSaveClub}
            disabled={isLoading || isSavingClub}
            className="bg-primary-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isSavingClub ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Save className="w-4 h-4" />
            )}
            Save Club Settings
          </button>
        </div>
        </details>
        {isSavingClub && <p role="status" className="mt-3 text-sm text-slate-600">Saving club settings…</p>}
        {clubMessage && (
          <div role={clubMessage.type === 'error' ? 'alert' : 'status'} className={`mt-3 p-3 rounded-lg text-sm ${
              clubMessage.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
            }`}>
            {clubMessage.text}
          </div>
        )}
      </section>

      {/* Send Announcement */}
      <section className="card p-4 sm:p-6" aria-labelledby="announcements-heading">
        <h2 id="announcements-heading" className="flex items-center gap-2 font-semibold text-slate-900"><Megaphone className="h-5 w-5 text-primary-700" aria-hidden="true" />Announcements</h2>
        <details className="mt-3">
          <summary className="min-h-11 cursor-pointer py-3 text-sm font-semibold text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600">Write an announcement</summary>
        <p className="text-sm text-slate-600 mb-4">
          Send a notification to all club members via push notification.
        </p>

        <div className="space-y-4">
          <div>
            <label htmlFor="announcement-title" className="block text-sm font-medium text-slate-700 mb-1">
              Title
            </label>
            <input
              type="text"
              id="announcement-title"
              maxLength={200}
              disabled={isSendingAnnouncement}
              value={announcementForm.title}
              onChange={(e) => setAnnouncementForm({ ...announcementForm, title: e.target.value })}
              placeholder="Announcement title"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div>
            <label htmlFor="announcement-message" className="block text-sm font-medium text-slate-700 mb-1">
              Message
            </label>
            <textarea
              id="announcement-message"
              maxLength={5000}
              disabled={isSendingAnnouncement}
              value={announcementForm.body}
              onChange={(e) => setAnnouncementForm({ ...announcementForm, body: e.target.value })}
              placeholder="Write your announcement message..."
              rows={4}
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>



          <button
            onClick={handleSendAnnouncement}
            disabled={isSendingAnnouncement || !announcementForm.title.trim() || !announcementForm.body.trim()}
            className="bg-secondary-500 text-white px-6 py-2 rounded-lg font-medium hover:bg-secondary-600 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isSendingAnnouncement ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Send className="w-4 h-4" />
            )}
            Publish announcement
          </button>
        </div>
        </details>
        {isSendingAnnouncement && <p role="status" className="mt-3 text-sm text-slate-600">Publishing announcement…</p>}
        {announcementMessage && (
          <div role={announcementMessage.type === 'error' ? 'alert' : 'status'} className={`mt-3 p-3 rounded-lg text-sm ${
              announcementMessage.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
            }`}>
            {announcementMessage.text}
          </div>
        )}
      </section>


    </div>
  );
}

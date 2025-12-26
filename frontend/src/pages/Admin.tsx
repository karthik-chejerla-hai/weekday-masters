import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Settings, Users, Calendar, Check, X, Loader2, MapPin, Save, Building } from 'lucide-react';
import { api } from '../services/api';
import type { User } from '../types';
import Avatar from '../components/ui/Avatar';

export default function Admin() {
  const [joinRequests, setJoinRequests] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [processingId, setProcessingId] = useState<string | null>(null);

  // Club settings state
  const [clubForm, setClubForm] = useState({ name: '', venue_name: '', venue_address: '' });
  const [isSavingClub, setIsSavingClub] = useState(false);
  const [clubMessage, setClubMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

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
      setClubForm({
        name: clubData.name || '',
        venue_name: clubData.venue_name || '',
        venue_address: clubData.venue_address || '',
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
    setIsSavingClub(true);
    setClubMessage(null);
    try {
      await api.updateClub(clubForm);
      setClubMessage({ type: 'success', text: 'Club settings saved!' });
    } catch (error) {
      console.error('Failed to save club:', error);
      setClubMessage({ type: 'error', text: 'Failed to save club settings' });
    } finally {
      setIsSavingClub(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          <Settings className="w-7 h-7 text-primary-600" />
          Admin Dashboard
        </h1>
        <p className="text-slate-600 mt-1">
          Manage club members and sessions
        </p>
      </div>

      <div className="grid sm:grid-cols-2 gap-4">
        <Link
          to="/admin/sessions"
          className="bg-white rounded-xl border border-slate-200 p-6 hover:shadow-md transition-shadow"
        >
          <Calendar className="w-10 h-10 text-primary-600 mb-3" />
          <h3 className="font-semibold text-slate-900">Manage Sessions</h3>
          <p className="text-sm text-slate-600 mt-1">Create and edit sessions</p>
        </Link>

        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <Users className="w-10 h-10 text-secondary-500 mb-3" />
          <h3 className="font-semibold text-slate-900">Pending Requests</h3>
          <p className="text-sm text-slate-600 mt-1">
            {joinRequests.length} request{joinRequests.length !== 1 ? 's' : ''} pending
          </p>
        </div>
      </div>

      {/* Club Settings */}
      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <h2 className="font-semibold text-slate-900 mb-4 flex items-center gap-2">
          <Building className="w-5 h-5 text-primary-600" />
          Club Settings
        </h2>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              Club Name
            </label>
            <input
              type="text"
              value={clubForm.name}
              onChange={(e) => setClubForm({ ...clubForm, name: e.target.value })}
              placeholder="Enter club name"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              <MapPin className="w-4 h-4 inline mr-1" />
              Venue Name
            </label>
            <input
              type="text"
              value={clubForm.venue_name}
              onChange={(e) => setClubForm({ ...clubForm, venue_name: e.target.value })}
              placeholder="e.g., Sydney Olympic Park Sports Centre"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              Venue Address
            </label>
            <textarea
              value={clubForm.venue_address}
              onChange={(e) => setClubForm({ ...clubForm, venue_address: e.target.value })}
              placeholder="Enter full venue address"
              rows={2}
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          {clubMessage && (
            <div className={`p-3 rounded-lg text-sm ${
              clubMessage.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
            }`}>
              {clubMessage.text}
            </div>
          )}

          <button
            onClick={handleSaveClub}
            disabled={isSavingClub}
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
      </div>

      {/* Join Requests */}
      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <h2 className="font-semibold text-slate-900 mb-4 flex items-center gap-2">
          <Users className="w-5 h-5 text-primary-600" />
          Join Requests
        </h2>

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
          </div>
        ) : joinRequests.length === 0 ? (
          <div className="text-center py-8 text-slate-500">
            <Users className="w-12 h-12 text-slate-300 mx-auto mb-3" />
            <p>No pending join requests</p>
          </div>
        ) : (
          <div className="space-y-3">
            {joinRequests.map((user) => (
              <div
                key={user.id}
                className="flex items-center justify-between p-4 bg-slate-50 rounded-lg"
              >
                <div className="flex items-center gap-3">
                  <Avatar src={user.profile_picture} name={user.name} />
                  <div>
                    <p className="font-medium text-slate-900">{user.name}</p>
                    <p className="text-sm text-slate-500">{user.email}</p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleApprove(user.id)}
                    disabled={processingId === user.id}
                    className="p-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 transition-colors disabled:opacity-50"
                    title="Approve"
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
                    className="p-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 transition-colors disabled:opacity-50"
                    title="Reject"
                  >
                    <X className="w-5 h-5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Mail, Phone, Shield, Save, Loader2, Bell, Smile, Settings, CalendarDays, UsersRound, ChevronRight } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import Avatar from '../components/ui/Avatar';
import Badge from '../components/ui/Badge';
import NotificationSettings from '../components/notifications/NotificationSettings';
import { displayName, firstName } from '../utils/members';

export default function Profile() {
  const { user, refreshUser, isAdmin } = useAuth();
  const [phoneNumber, setPhoneNumber] = useState(user?.phone_number || '');
  const [nickname, setNickname] = useState(user?.nickname || '');
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Left blank, the club sees their first name — so the placeholder shows what
  // they would be called rather than an empty box that says nothing.
  const defaultNickname = firstName(user?.name || '');

  const handleSave = async () => {
    setIsSaving(true);
    setMessage(null);
    try {
      await api.updateMe({ phone_number: phoneNumber, nickname });
      await refreshUser();
      setMessage({ type: 'success', text: 'Profile updated successfully!' });
    } catch (err) {
      const serverError = (err as { response?: { data?: { error?: string } } })?.response?.data
        ?.error;
      setMessage({ type: 'error', text: serverError || 'Failed to update profile' });
    } finally {
      setIsSaving(false);
    }
  };

  if (!user) return null;

  return (
    <div className="space-y-6">
      <div className="page-heading">
        <p className="page-kicker">Your account</p>
        <h1 className="page-title">Profile</h1>
        <p className="page-description">Manage how the club sees you and where notifications reach you.</p>
      </div>

      <div className="card p-5 sm:p-6">
        <div className="flex items-center gap-4 mb-6 pb-6 border-b border-slate-200">
          <Avatar src={user.profile_picture} name={displayName(user)} size="lg" />
          <div>
            <h2 className="text-xl font-semibold text-slate-900">{displayName(user)}</h2>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant={user.role === 'admin' ? 'info' : 'default'}>
                {user.role === 'admin' && <Shield className="w-3 h-3 mr-1" />}
                {user.role.charAt(0).toUpperCase() + user.role.slice(1)}
              </Badge>
              <Badge variant={user.membership_status === 'approved' ? 'success' : 'warning'}>
                {user.membership_status.charAt(0).toUpperCase() + user.membership_status.slice(1)}
              </Badge>
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              <Mail className="w-4 h-4 inline mr-2" />
              Email
            </label>
            <input
              type="email"
              value={user.email}
              disabled
              className="w-full px-4 py-2 rounded-lg border border-slate-300 bg-slate-50 text-slate-500"
            />
            <p className="text-xs text-slate-500 mt-1">Email is managed by Google and cannot be changed</p>
          </div>

          <div>
            <label htmlFor="nickname" className="block text-sm font-medium text-slate-700 mb-1">
              <Smile className="w-4 h-4 inline mr-2" />
              Nickname
            </label>
            <input
              id="nickname"
              type="text"
              value={nickname}
              maxLength={100}
              onChange={(e) => setNickname(e.target.value)}
              placeholder={defaultNickname}
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
            <p className="text-xs text-slate-500 mt-1">
              What the club sees you as — on session lists, balances and settlements. Leave it
              blank to go by {defaultNickname || 'your first name'}.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              <Phone className="w-4 h-4 inline mr-2" />
              Phone Number
            </label>
            <input
              type="tel"
              value={phoneNumber}
              onChange={(e) => setPhoneNumber(e.target.value)}
              placeholder="Enter your phone number"
              className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </div>

          {message && (
            <div className={`p-3 rounded-lg text-sm ${
              message.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
            }`}>
              {message.text}
            </div>
          )}

          <button
            onClick={handleSave}
            disabled={isSaving}
            className="btn-primary w-full gap-2 sm:w-auto"
          >
            {isSaving ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Save className="w-4 h-4" />
            )}
            Save Changes
          </button>
        </div>
      </div>

      {isAdmin && (
        <section aria-labelledby="admin-tools-heading">
          <div className="mb-3 flex items-center gap-2">
            <Settings className="h-5 w-5 text-primary-700" />
            <h2 id="admin-tools-heading" className="text-lg font-semibold text-slate-950">Club admin</h2>
          </div>
          <div className="card divide-y divide-slate-100">
            <AdminLink to="/admin" icon={Settings} label="Admin home" description="Join requests, settings and announcements" />
            <AdminLink to="/admin/sessions" icon={CalendarDays} label="Manage sessions" description="Schedule, edit and settle sessions" />
            <AdminLink to="/admin/members" icon={UsersRound} label="Manage members" description="Invite, edit or reinstate club members" />
          </div>
        </section>
      )}

      {/* Notification Settings */}
      <div>
        <h2 className="text-xl font-bold text-slate-900 flex items-center gap-2 mb-4">
          <Bell className="w-6 h-6 text-primary-600" />
          Notification Settings
        </h2>
        <NotificationSettings />
      </div>
    </div>
  );
}

function AdminLink({ to, icon: Icon, label, description }: {
  to: string;
  icon: typeof Settings;
  label: string;
  description: string;
}) {
  return (
    <Link to={to} className="flex min-h-[72px] items-center gap-3 px-4 py-3 transition-colors first:rounded-t-2xl last:rounded-b-2xl hover:bg-slate-50">
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700">
        <Icon className="h-5 w-5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-semibold text-slate-900">{label}</span>
        <span className="block truncate text-xs text-slate-500">{description}</span>
      </span>
      <ChevronRight className="h-5 w-5 text-slate-400" />
    </Link>
  );
}

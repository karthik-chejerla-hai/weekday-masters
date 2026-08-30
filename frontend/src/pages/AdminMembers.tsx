import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  UserPlus,
  Loader2,
  Pencil,
  Save,
  X,
  Trash2,
  RotateCcw,
  Mail,
  Phone,
  Shield,
  Search,
} from 'lucide-react';
import { api } from '../services/api';
import type { InviteMemberInput, User, UserRole } from '../types';
import { displayName, isPendingInvite } from '../utils/members';
import Avatar from '../components/ui/Avatar';
import Badge from '../components/ui/Badge';

/**
 * The member endpoints answer failures as `{ "error": "..." }`, and the messages
 * are written to be read by the admin who caused them — "settle up before
 * removing them", "this is the club's only admin". Showing the server's sentence
 * beats replacing it with a generic one.
 */
function readError(err: unknown, fallback: string): string {
  const response = (err as { response?: { data?: { error?: string } } })?.response;
  return response?.data?.error || fallback;
}

type Tab = 'members' | 'invited' | 'removed';

const tabs: Array<{ id: Tab; label: string }> = [
  { id: 'members', label: 'Members' },
  { id: 'invited', label: 'Invited' },
  { id: 'removed', label: 'Removed' },
];

/** Which tab a row belongs to. Pending and rejected join requests belong to the
 *  dashboard's approval queue, not here, so they fall through to null. */
function tabFor(user: User): Tab | null {
  if (user.membership_status === 'removed') return 'removed';
  if (user.membership_status !== 'approved') return null;
  return isPendingInvite(user) ? 'invited' : 'members';
}

const emptyInvite: InviteMemberInput = {
  email: '',
  name: '',
  nickname: '',
  phone_number: '',
  role: 'player',
};

export default function AdminMembers() {
  const navigate = useNavigate();

  const [members, setMembers] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [tab, setTab] = useState<Tab>('members');
  const [query, setQuery] = useState('');

  const [showInvite, setShowInvite] = useState(false);
  const [inviteForm, setInviteForm] = useState<InviteMemberInput>(emptyInvite);
  const [isInviting, setIsInviting] = useState(false);
  const [inviteError, setInviteError] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    loadMembers();
  }, []);

  const loadMembers = async () => {
    try {
      setMembers(await api.adminListMembers());
      setLoadError(null);
    } catch (err) {
      setLoadError(readError(err, 'Could not load the member list.'));
    } finally {
      setIsLoading(false);
    }
  };

  /** Replaces one row in place, so an edit does not cost a full reload. */
  const replaceMember = (updated: User) =>
    setMembers((prev) => prev.map((m) => (m.id === updated.id ? updated : m)));

  const counts = useMemo(() => {
    const tally: Record<Tab, number> = { members: 0, invited: 0, removed: 0 };
    for (const member of members) {
      const belongs = tabFor(member);
      if (belongs) tally[belongs] += 1;
    }
    return tally;
  }, [members]);

  const visible = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return members.filter((member) => {
      if (tabFor(member) !== tab) return false;
      if (!needle) return true;
      return (
        member.name.toLowerCase().includes(needle) ||
        member.nickname.toLowerCase().includes(needle) ||
        member.email.toLowerCase().includes(needle)
      );
    });
  }, [members, tab, query]);

  const handleInvite = async (event: React.FormEvent) => {
    event.preventDefault();
    setIsInviting(true);
    setInviteError(null);
    try {
      const invited = await api.inviteMember({
        ...inviteForm,
        email: inviteForm.email.trim(),
        name: inviteForm.name.trim(),
      });
      setMembers((prev) => [...prev, invited]);
      setInviteForm(emptyInvite);
      setShowInvite(false);
      setTab('invited');
      setNotice(
        `${displayName(invited)} has been added. They join properly the first time they sign in with ${invited.email}.`
      );
    } catch (err) {
      setInviteError(readError(err, 'Could not add this member.'));
    } finally {
      setIsInviting(false);
    }
  };

  const handleRemove = async (member: User) => {
    setBusyId(member.id);
    setRowError(null);
    setNotice(null);
    try {
      replaceMember(await api.removeMember(member.id));
      setNotice(`${displayName(member)} has been removed. Their history is kept, and you can put them back.`);
    } catch (err) {
      setRowError(readError(err, 'Could not remove this member.'));
    } finally {
      setBusyId(null);
    }
  };

  const handleReinstate = async (member: User) => {
    setBusyId(member.id);
    setRowError(null);
    setNotice(null);
    try {
      replaceMember(await api.reinstateMember(member.id));
      setNotice(`${displayName(member)} is back in the club.`);
    } catch (err) {
      setRowError(readError(err, 'Could not reinstate this member.'));
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <button
            onClick={() => navigate('/admin')}
            className="text-slate-500 hover:text-slate-700 flex items-center gap-1 text-sm mb-2"
          >
            <ArrowLeft className="w-4 h-4" />
            Admin
          </button>
          <h1 className="text-2xl font-bold text-slate-900">Members</h1>
          <p className="text-slate-600 mt-1 text-sm">
            Add someone before they sign up, fix their details, or take them out of the club.
            People who asked to join are approved on the{' '}
            <button onClick={() => navigate('/admin')} className="text-primary-600 underline">
              admin dashboard
            </button>
            .
          </p>
        </div>

        <button
          onClick={() => {
            setShowInvite((open) => !open);
            setInviteError(null);
          }}
          className="bg-primary-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors flex items-center gap-2 shrink-0"
        >
          <UserPlus className="w-4 h-4" />
          Add member
        </button>
      </div>

      {showInvite && (
        <form
          onSubmit={handleInvite}
          aria-label="Add a member"
          className="bg-white rounded-xl border border-slate-200 p-6 space-y-4"
        >
          <div>
            <h2 className="font-semibold text-slate-900">Add a member</h2>
            <p className="text-sm text-slate-600 mt-1">
              They are a member straight away — you can RSVP them into sessions and settle against
              them. The first time they sign in with this email address, this record becomes their
              account. No email is sent from here, so let them know yourself.
            </p>
          </div>

          <div className="grid sm:grid-cols-2 gap-4">
            <label className="block">
              <span className="block text-sm font-medium text-slate-700 mb-1">Email</span>
              <input
                type="email"
                required
                value={inviteForm.email}
                onChange={(e) => setInviteForm({ ...inviteForm, email: e.target.value })}
                placeholder="them@example.com"
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </label>

            <label className="block">
              <span className="block text-sm font-medium text-slate-700 mb-1">Full name</span>
              <input
                type="text"
                required
                value={inviteForm.name}
                onChange={(e) => setInviteForm({ ...inviteForm, name: e.target.value })}
                placeholder="Priya Raman"
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </label>

            <label className="block">
              <span className="block text-sm font-medium text-slate-700 mb-1">
                Nickname <span className="text-slate-400 font-normal">(optional)</span>
              </span>
              <input
                type="text"
                value={inviteForm.nickname}
                onChange={(e) => setInviteForm({ ...inviteForm, nickname: e.target.value })}
                placeholder="What the club calls them"
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </label>

            <label className="block">
              <span className="block text-sm font-medium text-slate-700 mb-1">
                Phone <span className="text-slate-400 font-normal">(optional)</span>
              </span>
              <input
                type="tel"
                value={inviteForm.phone_number}
                onChange={(e) => setInviteForm({ ...inviteForm, phone_number: e.target.value })}
                placeholder="+61 400 000 000"
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </label>

            <label className="block">
              <span className="block text-sm font-medium text-slate-700 mb-1">Role</span>
              <select
                value={inviteForm.role}
                onChange={(e) =>
                  setInviteForm({ ...inviteForm, role: e.target.value as 'player' | 'admin' })
                }
                className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              >
                <option value="player">Player</option>
                <option value="admin">Admin</option>
              </select>
            </label>
          </div>

          {inviteError && (
            <div role="alert" className="p-3 rounded-lg text-sm bg-red-50 text-red-700">
              {inviteError}
            </div>
          )}

          <div className="flex items-center gap-2">
            <button
              type="submit"
              disabled={isInviting}
              className="bg-primary-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {isInviting ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <UserPlus className="w-4 h-4" />
              )}
              Add member
            </button>
            <button
              type="button"
              onClick={() => setShowInvite(false)}
              className="px-4 py-2 rounded-lg font-medium text-slate-600 hover:bg-slate-100 transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {notice && (
        <div role="status" className="p-3 rounded-lg text-sm bg-green-50 text-green-700">
          {notice}
        </div>
      )}
      {rowError && (
        <div role="alert" className="p-3 rounded-lg text-sm bg-red-50 text-red-700">
          {rowError}
        </div>
      )}

      <div className="bg-white rounded-xl border border-slate-200">
        <div className="p-4 border-b border-slate-200 flex flex-wrap items-center gap-3 justify-between">
          <div className="flex gap-1" role="tablist">
            {tabs.map(({ id, label }) => (
              <button
                key={id}
                role="tab"
                aria-selected={tab === id}
                onClick={() => setTab(id)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  tab === id
                    ? 'bg-primary-50 text-primary-700'
                    : 'text-slate-600 hover:bg-slate-100'
                }`}
              >
                {label} ({counts[id]})
              </button>
            ))}
          </div>

          <label className="relative">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <span className="sr-only">Search members</span>
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search by name or email"
              className="pl-9 pr-4 py-2 rounded-lg border border-slate-300 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            />
          </label>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
          </div>
        ) : loadError ? (
          <div role="alert" className="p-6 text-center text-red-700">
            {loadError}
          </div>
        ) : visible.length === 0 ? (
          <div className="p-8 text-center text-slate-500">
            {query ? 'Nobody matches that search.' : 'Nobody here yet.'}
          </div>
        ) : (
          <ul className="divide-y divide-slate-200">
            {visible.map((member) => (
              <li key={member.id} className="p-4">
                {editingId === member.id ? (
                  <MemberEditor
                    member={member}
                    onCancel={() => setEditingId(null)}
                    onSaved={(updated) => {
                      replaceMember(updated);
                      setEditingId(null);
                      setNotice(`${displayName(updated)}'s details were updated.`);
                    }}
                  />
                ) : (
                  <MemberRow
                    member={member}
                    isBusy={busyId === member.id}
                    onEdit={() => {
                      setEditingId(member.id);
                      setRowError(null);
                      setNotice(null);
                    }}
                    onRemove={() => handleRemove(member)}
                    onReinstate={() => handleReinstate(member)}
                  />
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function MemberRow({
  member,
  isBusy,
  onEdit,
  onRemove,
  onReinstate,
}: {
  member: User;
  isBusy: boolean;
  onEdit: () => void;
  onRemove: () => void;
  onReinstate: () => void;
}) {
  const removed = member.membership_status === 'removed';

  return (
    <div className="flex items-center justify-between gap-4">
      <div className="flex items-center gap-3 min-w-0">
        <Avatar src={member.profile_picture} name={displayName(member)} />
        <div className="min-w-0">
          <p className="font-medium text-slate-900 flex items-center gap-2 flex-wrap">
            {displayName(member)}
            {member.nickname && (
              <span className="text-sm font-normal text-slate-500">{member.name}</span>
            )}
            {member.role === 'admin' && (
              <Badge variant="info">
                <Shield className="w-3 h-3 mr-1" />
                Admin
              </Badge>
            )}
            {isPendingInvite(member) && !removed && <Badge variant="warning">Not signed in</Badge>}
            {removed && <Badge variant="danger">Removed</Badge>}
          </p>
          <p className="text-sm text-slate-500 flex items-center gap-3 flex-wrap">
            <span className="flex items-center gap-1 truncate">
              <Mail className="w-3.5 h-3.5 shrink-0" />
              {member.email}
            </span>
            {member.phone_number && (
              <span className="flex items-center gap-1">
                <Phone className="w-3.5 h-3.5" />
                {member.phone_number}
              </span>
            )}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        {removed ? (
          <button
            onClick={onReinstate}
            disabled={isBusy}
            className="p-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 transition-colors disabled:opacity-50"
            title={`Reinstate ${displayName(member)}`}
            aria-label={`Reinstate ${displayName(member)}`}
          >
            {isBusy ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : (
              <RotateCcw className="w-5 h-5" />
            )}
          </button>
        ) : (
          <>
            <button
              onClick={onEdit}
              className="p-2 bg-slate-100 text-slate-700 rounded-lg hover:bg-slate-200 transition-colors"
              title={`Edit ${displayName(member)}`}
              aria-label={`Edit ${displayName(member)}`}
            >
              <Pencil className="w-5 h-5" />
            </button>
            <button
              onClick={onRemove}
              disabled={isBusy}
              className="p-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 transition-colors disabled:opacity-50"
              title={`Remove ${displayName(member)}`}
              aria-label={`Remove ${displayName(member)}`}
            >
              {isBusy ? (
                <Loader2 className="w-5 h-5 animate-spin" />
              ) : (
                <Trash2 className="w-5 h-5" />
              )}
            </button>
          </>
        )}
      </div>
    </div>
  );
}

function MemberEditor({
  member,
  onCancel,
  onSaved,
}: {
  member: User;
  onCancel: () => void;
  onSaved: (updated: User) => void;
}) {
  // An unclaimed invite is the only time the email is still ours to correct;
  // once Auth0 owns the identity, changing it here would not change how they
  // sign in. The backend enforces this too — the UI just does not offer it.
  const emailEditable = isPendingInvite(member);

  const [form, setForm] = useState({
    name: member.name,
    nickname: member.nickname,
    phone_number: member.phone_number,
    email: member.email,
    role: member.role as UserRole,
  });
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setIsSaving(true);
    setError(null);
    try {
      onSaved(
        await api.updateMember(member.id, {
          name: form.name,
          nickname: form.nickname,
          phone_number: form.phone_number,
          role: form.role,
          ...(emailEditable ? { email: form.email } : {}),
        })
      );
    } catch (err) {
      setError(readError(err, 'Could not save these details.'));
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} aria-label={`Edit ${displayName(member)}`} className="space-y-4">
      <div className="grid sm:grid-cols-2 gap-4">
        <label className="block">
          <span className="block text-sm font-medium text-slate-700 mb-1">Full name</span>
          <input
            type="text"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </label>

        <label className="block">
          <span className="block text-sm font-medium text-slate-700 mb-1">Nickname</span>
          <input
            type="text"
            value={form.nickname}
            onChange={(e) => setForm({ ...form, nickname: e.target.value })}
            placeholder="What the club calls them"
            className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </label>

        <label className="block">
          <span className="block text-sm font-medium text-slate-700 mb-1">Phone</span>
          <input
            type="tel"
            value={form.phone_number}
            onChange={(e) => setForm({ ...form, phone_number: e.target.value })}
            className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </label>

        <label className="block">
          <span className="block text-sm font-medium text-slate-700 mb-1">Role</span>
          <select
            value={form.role}
            onChange={(e) => setForm({ ...form, role: e.target.value as UserRole })}
            className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          >
            <option value="player">Player</option>
            <option value="admin">Admin</option>
          </select>
        </label>

        <label className="block sm:col-span-2">
          <span className="block text-sm font-medium text-slate-700 mb-1">Email</span>
          <input
            type="email"
            value={form.email}
            disabled={!emailEditable}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            className="w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent disabled:bg-slate-100 disabled:text-slate-500"
          />
          <span className="block text-xs text-slate-500 mt-1">
            {emailEditable
              ? 'They have not signed in yet, so this is still yours to correct. It is what their sign-in will be matched against.'
              : 'Set by their Google sign-in and not editable here.'}
          </span>
        </label>
      </div>

      {error && (
        <div role="alert" className="p-3 rounded-lg text-sm bg-red-50 text-red-700">
          {error}
        </div>
      )}

      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={isSaving}
          className="bg-primary-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center gap-2"
        >
          {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          Save
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 rounded-lg font-medium text-slate-600 hover:bg-slate-100 transition-colors flex items-center gap-2"
        >
          <X className="w-4 h-4" />
          Cancel
        </button>
      </div>
    </form>
  );
}

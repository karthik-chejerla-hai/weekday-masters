import { Clock, LogOut, UserX } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import Avatar from '../components/ui/Avatar';
import { displayName } from '../utils/members';

export default function PendingApproval() {
  const { user, logout } = useAuth();

  // Both statuses land here, because neither can see the club — but "we have
  // not got to you yet" and "you are no longer a member" are different things
  // to be told, and only one of them ends by itself.
  const removed = user?.membership_status === 'removed';

  return (
    <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
      <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-8 max-w-md w-full text-center">
        <div
          className={`w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-6 ${
            removed ? 'bg-slate-200' : 'bg-amber-100'
          }`}
        >
          {removed ? (
            <UserX className="w-8 h-8 text-slate-600" />
          ) : (
            <Clock className="w-8 h-8 text-amber-600" />
          )}
        </div>

        <h1 className="text-2xl font-bold text-slate-900 mb-2">
          {removed ? 'No Longer a Member' : 'Membership Pending'}
        </h1>
        <p className="text-slate-600 mb-6">
          {removed
            ? 'Your membership of the club has ended. If you think that is a mistake, ask a club admin — they can put you back, and your history is still there.'
            : "Your request to join the club is awaiting approval from an administrator. You'll be notified once your membership is approved."}
        </p>

        {user && (
          <div className="flex items-center gap-3 p-4 bg-slate-50 rounded-lg mb-6">
            <Avatar src={user.profile_picture} name={displayName(user)} />
            <div className="text-left">
              <p className="font-medium text-slate-900">{displayName(user)}</p>
              <p className="text-sm text-slate-500">{user.email}</p>
            </div>
          </div>
        )}

        <button
          onClick={logout}
          className="w-full flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-slate-600 hover:bg-slate-100 transition-colors"
        >
          <LogOut className="w-4 h-4" />
          Sign Out
        </button>
      </div>
    </div>
  );
}

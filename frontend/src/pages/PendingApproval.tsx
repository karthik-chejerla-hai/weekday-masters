import { Clock, LogOut } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import Avatar from '../components/ui/Avatar';

export default function PendingApproval() {
  const { user, logout } = useAuth();

  return (
    <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
      <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-8 max-w-md w-full text-center">
        <div className="w-16 h-16 bg-amber-100 rounded-full flex items-center justify-center mx-auto mb-6">
          <Clock className="w-8 h-8 text-amber-600" />
        </div>

        <h1 className="text-2xl font-bold text-slate-900 mb-2">
          Membership Pending
        </h1>
        <p className="text-slate-600 mb-6">
          Your request to join the club is awaiting approval from an administrator.
          You'll be notified once your membership is approved.
        </p>

        {user && (
          <div className="flex items-center gap-3 p-4 bg-slate-50 rounded-lg mb-6">
            <Avatar src={user.profile_picture} name={user.name} />
            <div className="text-left">
              <p className="font-medium text-slate-900">{user.name}</p>
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

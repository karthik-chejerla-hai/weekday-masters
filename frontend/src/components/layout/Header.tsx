import { Link } from 'react-router-dom';
import { LogOut } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import Avatar from '../ui/Avatar';

export default function Header() {
  const { user, logout, isAdmin } = useAuth();

  return (
    <header className="bg-white border-b border-slate-200 sticky top-0 z-40">
      <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
        <Link to="/dashboard" className="flex items-center gap-2">
          <div className="w-10 h-10 bg-primary-600 rounded-xl flex items-center justify-center text-2xl">
            🏸
          </div>
          <span className="font-bold text-lg text-slate-900 hidden sm:block">
            Weekday Masters
          </span>
        </Link>

        <div className="flex items-center gap-4">
          {isAdmin && (
            <Link
              to="/admin"
              className="text-sm font-medium text-primary-600 hover:text-primary-700 hidden md:block"
            >
              Admin
            </Link>
          )}

          <Link to="/profile" className="flex items-center gap-2">
            <Avatar src={user?.profile_picture} name={user?.name || ''} size="sm" />
            <span className="text-sm font-medium text-slate-700 hidden sm:block">
              {user?.name}
            </span>
          </Link>

          <button
            onClick={logout}
            className="p-2 text-slate-500 hover:text-slate-700 hover:bg-slate-100 rounded-lg transition-colors"
            title="Logout"
          >
            <LogOut className="w-5 h-5" />
          </button>
        </div>
      </div>
    </header>
  );
}

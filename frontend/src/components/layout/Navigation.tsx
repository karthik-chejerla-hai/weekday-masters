import { NavLink } from 'react-router-dom';
import { Home, Calendar, User, Settings } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

export default function Navigation() {
  const { isAdmin } = useAuth();

  const navItems = [
    { to: '/dashboard', icon: Home, label: 'Home' },
    { to: '/sessions', icon: Calendar, label: 'Sessions' },
    { to: '/profile', icon: User, label: 'Profile' },
    ...(isAdmin ? [{ to: '/admin', icon: Settings, label: 'Admin' }] : []),
  ];

  return (
    <nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-slate-200 md:hidden z-50">
      <div className="flex items-center justify-around h-16">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              `flex flex-col items-center justify-center w-full h-full transition-colors ${
                isActive
                  ? 'text-primary-600'
                  : 'text-slate-500 hover:text-slate-700'
              }`
            }
          >
            <Icon className="w-6 h-6" />
            <span className="text-xs mt-1">{label}</span>
          </NavLink>
        ))}
      </div>
    </nav>
  );
}

import { NavLink } from 'react-router-dom';
import { Home, CalendarDays, User, WalletCards } from 'lucide-react';

export default function Navigation() {
  const navItems = [
    { to: '/dashboard', icon: Home, label: 'Home' },
    { to: '/sessions', icon: CalendarDays, label: 'Sessions' },
    { to: '/money', icon: WalletCards, label: 'Money' },
    { to: '/profile', icon: User, label: 'Profile' },
  ];

  return (
    <nav aria-label="Mobile navigation" className="fixed inset-x-0 bottom-0 z-50 border-t border-slate-200 bg-white/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden">
      <div className="mx-auto grid h-16 max-w-lg grid-cols-4 px-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              `relative flex min-w-0 flex-col items-center justify-center gap-1 rounded-xl text-xs font-medium transition-colors ${
                isActive
                  ? 'text-primary-800'
                  : 'text-slate-500 hover:text-slate-800'
              }`
            }
          >
            {({ isActive }) => (
              <>
                {isActive && <span className="absolute top-0 h-0.5 w-8 rounded-full bg-primary-700" />}
                <Icon className="h-5 w-5" />
                <span>{label}</span>
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}

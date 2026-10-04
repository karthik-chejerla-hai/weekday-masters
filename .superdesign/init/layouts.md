# Shared Layouts

## Layout

Path: `frontend/src/components/layout/Layout.tsx`

```tsx
import { Outlet } from 'react-router-dom';
import Navigation from './Navigation';
import Header from './Header';

export default function Layout() {
  return (
    <div className="min-h-screen bg-slate-50 pb-20 md:pb-0">
      <Header />
      <main className="max-w-4xl mx-auto px-4 py-6">
        <Outlet />
      </main>
      <Navigation />
    </div>
  );
}
```

## Header

Path: `frontend/src/components/layout/Header.tsx`

```tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { LogOut } from 'lucide-react';
import { useAuth } from '../../context/useAuth';
import { api } from '../../services/api';
import Avatar from '../ui/Avatar';
import BalanceChip from '../money/BalanceChip';
import type { MyBalance } from '../../types';
import { displayName } from '../../utils/members';

export default function Header() {
  const { user, logout, isAdmin, isApproved } = useAuth();
  const [balance, setBalance] = useState<MyBalance | null>(null);
  useEffect(() => {
    if (!isApproved) return;
    let cancelled = false;
    api.getMyBalance().then((result) => { if (!cancelled) setBalance(result); }).catch(() => {});
    return () => { cancelled = true; };
  }, [isApproved]);
  return (
    <header className="bg-white border-b border-slate-200 sticky top-0 z-40">
      <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
        <Link to="/dashboard" className="flex items-center gap-2">
          <div className="w-10 h-10 bg-primary-600 rounded-xl flex items-center justify-center text-2xl">🏸</div>
          <span className="font-bold text-lg text-slate-900 hidden sm:block">Rally</span>
        </Link>
        <div className="flex items-center gap-4">
          {isAdmin && <Link to="/admin" className="text-sm font-medium text-primary-600 hover:text-primary-700 hidden md:block">Admin</Link>}
          {balance && <Link to="/money" aria-label="Your balance"><BalanceChip cents={balance.balance_cents} state={balance.state} compact /></Link>}
          <Link to="/profile" className="flex items-center gap-2">
            <Avatar src={user?.profile_picture} name={displayName(user)} size="sm" />
            <span className="text-sm font-medium text-slate-700 hidden sm:block">{displayName(user)}</span>
          </Link>
          <button onClick={logout} className="p-2 text-slate-500 hover:text-slate-700 hover:bg-slate-100 rounded-lg transition-colors" title="Logout"><LogOut className="w-5 h-5" /></button>
        </div>
      </div>
    </header>
  );
}
```

## Navigation

Path: `frontend/src/components/layout/Navigation.tsx`

```tsx
import { NavLink } from 'react-router-dom';
import { Home, Calendar, User, Settings, Wallet } from 'lucide-react';
import { useAuth } from '../../context/useAuth';

export default function Navigation() {
  const { isAdmin } = useAuth();
  const navItems = [
    { to: '/dashboard', icon: Home, label: 'Home' },
    { to: '/sessions', icon: Calendar, label: 'Sessions' },
    { to: '/money', icon: Wallet, label: 'Money' },
    { to: '/profile', icon: User, label: 'Profile' },
    ...(isAdmin ? [{ to: '/admin', icon: Settings, label: 'Admin' }] : []),
  ];
  return (
    <nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-slate-200 md:hidden z-50">
      <div className="flex items-center justify-around h-16">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink key={to} to={to} className={({ isActive }) => `flex flex-col items-center justify-center w-full h-full transition-colors ${isActive ? 'text-primary-600' : 'text-slate-500 hover:text-slate-700'}`}>
            <Icon className="w-6 h-6" /><span className="text-xs mt-1">{label}</span>
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
```


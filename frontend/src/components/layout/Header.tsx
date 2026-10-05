import { useEffect, useState } from 'react';
import { Link, NavLink } from 'react-router-dom';
import { CalendarDays, Eye, Home, LogOut, Settings, UsersRound, WalletCards, MessageCircle } from 'lucide-react';
import { useAuth } from '../../context/useAuth';
import { api } from '../../services/api';
import Avatar from '../ui/Avatar';
import BalanceChip from '../money/BalanceChip';
import type { MyBalance } from '../../types';
import { displayName } from '../../utils/members';

export default function Header() {
  const { user, logout, isAdmin, isApproved, startMemberPreview } = useAuth();
  const [balance, setBalance] = useState<MyBalance | null>(null);

  const [balanceVersion, setBalanceVersion] = useState(0);
  useEffect(() => {
    const refresh = () => setBalanceVersion((version) => version + 1);
    window.addEventListener('rally:balances-changed', refresh);
    return () => window.removeEventListener('rally:balances-changed', refresh);
  }, []);

  // The number people check most often, so it lives where they already look.
  // A chip that is red every time you open the app does more than a reminder
  // email ever will.
  useEffect(() => {
    if (!isApproved) return;
    let cancelled = false;
    api
      .getMyBalance()
      .then((result) => {
        if (!cancelled) setBalance(result);
      })
      .catch(() => {
        // A missing balance is not worth breaking the header over.
      });
    return () => {
      cancelled = true;
    };
  }, [isApproved, balanceVersion]);

  const primaryItems = [
    { to: '/dashboard', icon: Home, label: 'Home' },
    { to: '/sessions', icon: CalendarDays, label: 'Sessions' },
    { to: '/money', icon: WalletCards, label: 'Money' },
    { to: '/assistant', icon: MessageCircle, label: 'Ask Rally' },
  ];

  const adminItems = [
    { to: '/admin', icon: Settings, label: 'Admin', end: true },
    { to: '/admin/sessions', icon: CalendarDays, label: 'Manage sessions' },
    { to: '/admin/members', icon: UsersRound, label: 'Manage members' },
  ];

  const navClass = ({ isActive }: { isActive: boolean }) =>
    `flex min-h-11 items-center gap-3 rounded-xl px-3 text-sm font-medium transition-colors ${
      isActive
        ? 'bg-primary-50 text-primary-800'
        : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
    }`;

  return (
    <>
      <header className="sticky top-0 z-40 border-b border-slate-200 bg-white/95 backdrop-blur md:hidden">
        <div className="flex h-16 items-center justify-between px-4">
          <Link to="/dashboard" className="flex items-center gap-2" aria-label="Rally home">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 ring-1 ring-primary-100">
              <img src="/badminton.svg" alt="" className="h-8 w-8" />
            </span>
            <span className="text-lg font-bold tracking-tight text-slate-950">Rally</span>
          </Link>

          <div className="flex items-center gap-2">
            {balance && (
              <Link to="/money" aria-label="Your balance" className="rounded-full focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2">
                <BalanceChip cents={balance.balance_cents} state={balance.state} compact />
              </Link>
            )}
            <Link to="/profile" aria-label={`Open ${displayName(user)}'s profile`} className="rounded-full focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2">
              <Avatar src={user?.profile_picture} name={displayName(user)} size="sm" />
            </Link>
          </div>
        </div>
      </header>

      <aside className="fixed inset-y-0 left-0 z-40 hidden w-64 flex-col border-r border-slate-200 bg-white p-4 md:flex">
        <Link to="/dashboard" className="flex items-center gap-3 px-2 py-2" aria-label="Rally home">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 ring-1 ring-primary-100">
            <img src="/badminton.svg" alt="" className="h-8 w-8" />
          </span>
          <div>
            <p className="font-bold tracking-tight text-slate-950">Rally</p>
            <p className="text-xs text-slate-500">Club hub</p>
          </div>
        </Link>

        <nav aria-label="Main navigation" className="mt-6 space-y-1">
          {primaryItems.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} className={navClass}>
              <Icon className="h-5 w-5" />
              <span>{label}</span>
            </NavLink>
          ))}
        </nav>

        {isAdmin && (
          <div className="mt-7">
            <p className="px-3 text-xs font-semibold uppercase tracking-wider text-slate-400">Club admin</p>
            <nav aria-label="Admin navigation" className="mt-2 space-y-1">
              {adminItems.map(({ to, icon: Icon, label, end }) => (
                <NavLink key={to} to={to} end={end} className={navClass}>
                  <Icon className="h-5 w-5" />
                  <span>{label}</span>
                </NavLink>
              ))}
            </nav>
            <button onClick={startMemberPreview} className="mt-2 flex min-h-11 w-full items-center gap-3 rounded-xl px-3 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900">
              <Eye className="h-5 w-5" />
              Preview as member
            </button>
          </div>
        )}

        <div className="mt-auto space-y-3 border-t border-slate-200 pt-4">
          {balance && (
            <Link to="/money" className="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-3">
              <span className="text-xs font-medium text-slate-500">Your balance</span>
              <BalanceChip cents={balance.balance_cents} state={balance.state} compact />
            </Link>
          )}
          <Link to="/profile" className="flex min-h-11 items-center gap-3 rounded-xl px-2 hover:bg-slate-50">
            <Avatar src={user?.profile_picture} name={displayName(user)} size="sm" />
            <span className="min-w-0 flex-1 truncate text-sm font-medium text-slate-800">{displayName(user)}</span>
          </Link>
          <button onClick={logout} className="flex min-h-11 w-full items-center gap-3 rounded-xl px-3 text-sm font-medium text-slate-500 hover:bg-slate-100 hover:text-slate-800" title="Logout">
            <LogOut className="h-5 w-5" />
            Sign out
          </button>
        </div>
      </aside>
    </>
  );
}

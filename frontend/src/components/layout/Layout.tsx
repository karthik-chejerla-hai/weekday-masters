import { Outlet } from 'react-router-dom';
import Navigation from './Navigation';
import Header from './Header';
import { Eye } from 'lucide-react';
import { useAuth } from '../../context/useAuth';

export default function Layout() {
  const { isViewingAsMember, stopMemberPreview } = useAuth();

  return (
    <div className="min-h-screen bg-slate-50 pb-[calc(5rem+env(safe-area-inset-bottom))] md:pb-0 md:pl-64">
      <Header />
      {isViewingAsMember && (
        <div className="border-b border-amber-200 bg-amber-50 px-4 py-2.5 sm:px-6 lg:px-10" role="status">
          <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2 text-sm text-amber-900">
              <Eye className="h-4 w-4 shrink-0" />
              <span className="truncate"><strong>Previewing member view.</strong> Admin-only controls are hidden.</span>
            </div>
            <button onClick={stopMemberPreview} className="min-h-10 shrink-0 rounded-lg px-3 text-sm font-semibold text-amber-900 hover:bg-amber-100">
              Exit preview
            </button>
          </div>
        </div>
      )}
      <main className="mx-auto w-full max-w-6xl px-4 py-5 sm:px-6 md:py-8 lg:px-10">
        <Outlet />
      </main>
      <Navigation />
    </div>
  );
}

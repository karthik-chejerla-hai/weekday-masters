import { Outlet } from 'react-router-dom';
import Navigation from './Navigation';
import Header from './Header';

export default function Layout() {
  return (
    <div className="min-h-screen bg-slate-50 pb-[calc(5rem+env(safe-area-inset-bottom))] md:pb-0 md:pl-64">
      <Header />
      <main className="mx-auto w-full max-w-6xl px-4 py-5 sm:px-6 md:py-8 lg:px-10">
        <Outlet />
      </main>
      <Navigation />
    </div>
  );
}

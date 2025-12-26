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

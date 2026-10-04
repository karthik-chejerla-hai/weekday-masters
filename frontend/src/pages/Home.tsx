import { useEffect, useState } from 'react';
import { Users, Calendar, Trophy } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Club } from '../types';

export default function Home() {
  const { login, authError, retryAuth } = useAuth();
  const [club, setClub] = useState<Club | null>(null);

  useEffect(() => {
    api.getClub().then(setClub).catch(console.error);
  }, []);

  return (
    <div className="min-h-screen bg-slate-50">
      <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 sm:py-14">
        <header className="flex items-center gap-3">
          <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary-50 ring-1 ring-primary-100">
            <img src="/badminton.svg" alt="" className="h-9 w-9" />
          </span>
          <span className="text-lg font-bold tracking-tight text-slate-950">Rally</span>
        </header>

        <main className="py-12 text-center sm:py-20">
          <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-3xl bg-primary-50 ring-1 ring-primary-100">
            <img src="/badminton.svg" alt="" className="h-16 w-16" />
          </div>
          <h1 className="text-4xl font-semibold tracking-tight text-slate-950 sm:text-5xl">
            {club?.name || 'Rally'}
          </h1>
          <p className="mx-auto mt-4 max-w-xl text-lg leading-8 text-slate-600">
            Everything your badminton club needs to organise games, track RSVPs, and stay connected.
          </p>
          <button
            onClick={login}
            className="btn-primary mt-8 px-7"
          >
            Sign in with Google
          </button>
          {authError && <div role="alert" className="mx-auto mt-4 max-w-xl rounded-lg bg-red-50 p-3 text-sm text-red-700"><p>{authError}</p>{retryAuth && <button type="button" onClick={retryAuth} className="mt-2 font-medium underline">Try again</button>}</div>}
        </main>

        <section className="grid gap-3 md:grid-cols-3">
          <FeatureCard
            icon={Calendar}
            title="Weekly Sessions"
            description="Join our regular weekly sessions and one-off games"
          />
          <FeatureCard
            icon={Users}
            title="Easy RSVP"
            description="Quickly confirm your attendance for upcoming sessions"
          />
          <FeatureCard
            icon={Trophy}
            title="Friendly Community"
            description="Play with players of all skill levels in a welcoming environment"
          />
        </section>

        {club?.venue_name && (
          <section className="card mt-6 p-5 text-center">
            <h2 className="font-semibold text-slate-950">Our venue</h2>
            <p className="mt-1 text-sm text-slate-700">{club.venue_name}</p>
            {club.venue_address && (
              <p className="mt-1 text-sm text-slate-500">{club.venue_address}</p>
            )}
          </section>
        )}
      </div>
    </div>
  );
}

function FeatureCard({ icon: Icon, title, description }: {
  icon: typeof Calendar;
  title: string;
  description: string;
}) {
  return (
    <div className="card p-5 text-left">
      <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 text-primary-700">
        <Icon className="h-5 w-5" />
      </span>
      <h3 className="mt-4 font-semibold text-slate-950">{title}</h3>
      <p className="mt-1 text-sm leading-6 text-slate-600">{description}</p>
    </div>
  );
}

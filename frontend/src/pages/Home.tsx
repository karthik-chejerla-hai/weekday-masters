import { useEffect, useState } from 'react';
import { Users, Calendar, Trophy } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { api } from '../services/api';
import type { Club } from '../types';

export default function Home() {
  const { login } = useAuth();
  const [club, setClub] = useState<Club | null>(null);

  useEffect(() => {
    api.getClub().then(setClub).catch(console.error);
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-b from-primary-600 to-primary-800">
      <div className="max-w-4xl mx-auto px-4 py-12">
        <div className="text-center text-white mb-12">
          <div className="w-20 h-20 bg-white/20 rounded-2xl flex items-center justify-center mx-auto mb-6 text-5xl">
            🏸
          </div>
          <h1 className="text-4xl font-bold mb-4">
            {club?.name || 'Weekday Masters'}
          </h1>
          <p className="text-xl text-primary-100 mb-8">
            Join our badminton community and play with us!
          </p>
          <button
            onClick={login}
            className="bg-white text-primary-700 px-8 py-4 rounded-xl font-semibold text-lg hover:bg-primary-50 transition-colors shadow-lg"
          >
            Sign in with Google
          </button>
        </div>

        <div className="grid md:grid-cols-3 gap-6 mb-12">
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
        </div>

        {club?.venue_name && (
          <div className="bg-white/10 backdrop-blur rounded-xl border border-white/20 p-6 text-white text-center">
            <h2 className="font-semibold text-lg mb-2">Our Venue</h2>
            <p className="text-primary-100">{club.venue_name}</p>
            {club.venue_address && (
              <p className="text-primary-200 text-sm mt-1">{club.venue_address}</p>
            )}
          </div>
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
    <div className="bg-white/10 backdrop-blur rounded-xl border border-white/20 p-6 text-white text-center">
      <Icon className="w-10 h-10 mx-auto mb-4 text-secondary-400" />
      <h3 className="font-semibold text-lg mb-2">{title}</h3>
      <p className="text-primary-100 text-sm">{description}</p>
    </div>
  );
}

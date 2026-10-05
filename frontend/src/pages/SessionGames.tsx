import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, Trophy } from 'lucide-react';
import { api } from '../services/api';
import { assistantError } from '../components/assistant/errors';
import AssistantPanel from '../components/assistant/AssistantPanel';
import GameForm from '../components/games/GameForm';
import GameCard from '../components/games/GameCard';
import type { GameList, Session } from '../types';

export default function SessionGames() {
  const { id = '' } = useParams();
  return <SessionGamesContent key={id} id={id} />;
}
function SessionGamesContent({ id }: { id: string }) {
  const [session, setSession] = useState<Session | null>(null);
  const [games, setGames] = useState<GameList | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState(0);
  const refresh = useCallback(() => { setOffset(0); setRevision((value) => value + 1); }, []);
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    setLoading(true); setError('');
    Promise.all([api.getSession(id), api.listGames(id, offset, controller.signal)]).then(([detail, results]) => {
      if (active) { setSession(detail.session); setGames(results); }
    }).catch((err) => { if (active) setError(assistantError(err, 'Could not load this session and its games.')); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; controller.abort(); };
  }, [id, offset, revision]);
  const canRecord = !!session && session.status !== 'cancelled' && !!session.starts_at && new Date(session.starts_at) <= new Date();
  return <div className="mx-auto max-w-3xl space-y-6">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <Link to={`/sessions/${id}`} className="inline-flex items-center gap-2 text-sm text-slate-600"><ArrowLeft className="h-4 w-4" /> Session</Link>
      <Link to="/games" className="btn-outline gap-2"><Trophy className="h-4 w-4" /> Head-to-head</Link>
    </div>
    <header><p className="text-sm font-medium text-primary-700">{session?.title ?? 'Session'}</p><h1 className="mt-1 text-3xl font-bold text-slate-950">Game scores</h1><p className="mt-2 text-slate-600">Four players. One score. A record to settle the debate.</p></header>
    {error && <div role="alert" className="card p-4 text-red-700">{error} <button className="underline" onClick={() => setRevision((n) => n + 1)}>Retry</button></div>}
    {canRecord && <>
      <AssistantPanel sessionId={id} mode="games" onGameSaved={refresh} />
      <details className="card p-5"><summary className="cursor-pointer font-semibold text-slate-900 min-h-8">Enter a score by hand</summary><div className="mt-4"><GameForm sessionId={id} onSaved={refresh} /></div></details>
    </>}
    {session && !canRecord && <p role="status" className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">{session.status === 'cancelled' ? 'This session is cancelled. Existing results remain available below.' : 'Score entry opens when this session starts.'}</p>}
    <section aria-label="Session game history" className="space-y-4">
      <h2 className="text-xl font-semibold text-slate-950">Recorded games{games ? ` (${games.total})` : ''}</h2>
      {loading ? <p role="status" className="text-slate-600">Loading games…</p> : games && <>
        {games.items.length === 0 && <div className="card p-6 text-center text-slate-600">No games recorded on this page.</div>}
        {games.items.map((game) => <GameCard key={`${game.id}-${game.version}`} game={game} onChanged={refresh} />)}
        {games.total > 50 && <div className="flex justify-between"><button className="btn-outline" disabled={offset === 0} onClick={() => setOffset((n) => Math.max(0, n - 50))}>Previous</button><button className="btn-outline" disabled={offset + 50 >= games.total} onClick={() => setOffset((n) => n + 50)}>Next</button></div>}
      </>}
    </section>
  </div>;
}

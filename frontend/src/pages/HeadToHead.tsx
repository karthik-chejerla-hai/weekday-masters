import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Trophy, Loader2 } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../context/useAuth';
import { assistantError } from '../components/assistant/errors';
import GameCard from '../components/games/GameCard';
import type { GameComparison, GamePlayer } from '../types';

export default function HeadToHead() {
  const { user } = useAuth();
  const [players, setPlayers] = useState<GamePlayer[]>([]);
  const [mode, setMode] = useState<'players' | 'teams'>('players');
  const [selected, setSelected] = useState([user?.id ?? '', '', '', '']);
  const [result, setResult] = useState<GameComparison | null>(null);
  const [offset, setOffset] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [loadAttempt, setLoadAttempt] = useState(0);
  const request = useRef<AbortController | null>(null);
  useEffect(() => {
    let active = true;
    api.gamePlayers().then((items) => { if (active) { setPlayers(items); setError(''); } }).catch(() => { if (active) setError('Could not load players. Try again.'); });
    return () => { active = false; request.current?.abort(); };
  }, [loadAttempt]);
  const clear = () => { request.current?.abort(); setBusy(false); setResult(null); setError(''); setOffset(0); };
  const compare = async (page = 0) => {
    request.current?.abort();
    const a = mode === 'players' ? [selected[0]] : selected.slice(0, 2);
    const b = mode === 'players' ? [selected[2]] : selected.slice(2);
    const ids = [...a, ...b];
    if (ids.some((id) => !id) || new Set(ids).size !== ids.length) { setError('Choose different players on each side.'); return; }
    const controller = new AbortController(); request.current = controller;
    setBusy(true); setError(''); setResult(null);
    try { const data = await api.headToHead(a, b, page, controller.signal); if (!controller.signal.aborted) { setResult(data); setOffset(page); } }
    catch (err) { if (!controller.signal.aborted) setError(assistantError(err, 'Could not load this comparison. Try again.')); }
    finally { if (!controller.signal.aborted) setBusy(false); }
  };
  const sideName = (side: number) => selected.slice(side * 2, side * 2 + (mode === 'players' ? 1 : 2)).map((id) => players.find((p) => p.id === id)?.name ?? 'Player').join(' + ');
  return <div className="mx-auto max-w-3xl space-y-6">
    <header className="space-y-3"><div className="inline-flex rounded-2xl bg-secondary-100 p-3 text-secondary-800"><Trophy className="h-7 w-7" /></div><h1 className="text-3xl font-bold text-slate-950">Head-to-head</h1><p className="text-slate-600">The score settles it. Compare opponents across recorded doubles games.</p><Link to="/sessions" className="inline-block text-sm font-medium text-primary-700">Choose a session to record a game</Link></header>
    <section className="card p-5 space-y-5" aria-label="Choose opponents">
      <div className="flex gap-2">{(['players', 'teams'] as const).map((value) => <button type="button" key={value} aria-pressed={mode === value} className={mode === value ? 'btn-primary flex-1' : 'btn-outline flex-1'} onClick={() => { clear(); setMode(value); }}>{value === 'players' ? 'Players' : 'Exact teams'}</button>)}</div>
      <p className="text-sm text-slate-500">{mode === 'players' ? 'Counts games played on opposite sides, with any partners.' : 'Counts only these two exact pairs. Partner order does not matter.'}</p>
      <div className="grid gap-4 sm:grid-cols-2">{['A', 'B'].map((side, sideIndex) => <fieldset key={side} className="space-y-3 rounded-xl bg-slate-50 p-3"><legend className="px-1 text-sm font-semibold text-slate-700">Side {side}</legend>
        {Array.from({ length: mode === 'players' ? 1 : 2 }, (_, slot) => {
          const index = sideIndex * 2 + slot;
          return <div key={index}><label htmlFor={`opponent-${index}`} className="label">Side {side} player {slot + 1}</label><select id={`opponent-${index}`} className="input min-h-12" value={selected[index]} onChange={(event) => { clear(); setSelected((old) => old.map((id, i) => i === index ? event.target.value : id)); }}><option value="">Choose player</option>{players.map((p) => <option value={p.id} key={p.id}>{players.filter((candidate) => candidate.name === p.name).length > 1 && p.full_name ? `${p.name} (${p.full_name})` : p.name}</option>)}</select></div>;
        })}
      </fieldset>)}</div>
      {error && <p role="alert" className="text-sm text-red-700">{error}{players.length === 0 && <button className="ml-2 underline" onClick={() => setLoadAttempt((n) => n + 1)}>Retry players</button>}</p>}
      <button className="btn-primary w-full min-h-12 gap-2" disabled={busy} onClick={() => void compare()}>{busy && <Loader2 className="h-4 w-4 animate-spin" />}{busy ? 'Loading record…' : 'Show record'}</button>
    </section>
    {result && <section aria-label="Head-to-head record" className="space-y-4">
      <div className="card p-5"><p className="text-center text-xs font-semibold uppercase tracking-wider text-slate-500">{result.total} recorded {result.total === 1 ? 'game' : 'games'}</p>
        <div className="mt-4 grid grid-cols-2 gap-4 text-center">{[0, 1].map((side) => <div key={side}><h2 className="font-semibold text-slate-800">{sideName(side)}</h2><p className="mt-2 text-5xl font-bold tabular-nums text-primary-800">{side === 0 ? result.wins_a : result.wins_b}</p><p className="mt-1 text-sm text-slate-500">wins · {side === 0 ? result.wins_b : result.wins_a} losses</p><p className="mt-2 text-sm font-medium text-slate-600">{side === 0 ? result.points_a : result.points_b} points</p></div>)}</div>
      </div>
      <h2 className="text-xl font-semibold text-slate-950">Game history</h2>
      {result.total === 0 && <p className="card p-6 text-center text-slate-600">No games between these opponents yet.</p>}
      {result.items.map((game) => <GameCard key={`${game.id}-${game.version}`} game={game} onChanged={() => void compare(0)} />)}
      {result.total > 50 && <div className="flex justify-between"><button className="btn-outline" disabled={busy || offset === 0} onClick={() => void compare(offset - 50)}>Previous</button><button className="btn-outline" disabled={busy || offset + 50 >= result.total} onClick={() => void compare(offset + 50)}>Next</button></div>}
    </section>}
  </div>;
}

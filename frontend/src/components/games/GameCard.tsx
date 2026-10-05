import { useState } from 'react';
import { Link } from 'react-router-dom';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../services/api';
import { useAuth } from '../../context/useAuth';
import { assistantError } from '../assistant/errors';
import type { GameResult } from '../../types';
import GameForm from './GameForm';

export default function GameCard({ game, onChanged }: { game: GameResult; onChanged: () => void }) {
  const { user, isAdmin } = useAuth();
  const [editing, setEditing] = useState(false);
  const [confirmVoid, setConfirmVoid] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [history, setHistory] = useState<GameResult[] | null>(null);
  const canEdit = !game.voided_at && (isAdmin || user?.id === game.created_by);
  const names = (team: GameResult['team_a']) => team.map((p) => p.name).join(' + ');
  const change = () => { setEditing(false); setHistory(null); onChanged(); };
  const voidResult = async () => {
    setBusy(true); setError('');
    try { await api.voidGame(game.id, game.version); setConfirmVoid(false); change(); }
    catch (err) { setError(assistantError(err, 'Could not void the result. Try again.')); }
    finally { setBusy(false); }
  };
  const showHistory = async () => {
    if (history) { setHistory(null); return; }
    setBusy(true); setError('');
    try { setHistory(await api.gameRevisions(game.id)); }
    catch (err) { setError(assistantError(err, 'Could not load result history.')); }
    finally { setBusy(false); }
  };
  return <article className="card space-y-4 p-4 sm:p-5">
    <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
      <Link className="font-medium text-primary-700 hover:underline" to={`/sessions/${game.session_id}/games`}>{game.session_title} · {game.session_date}</Link>
      {game.voided_at && <span className="rounded-full bg-slate-100 px-2 py-1 font-semibold text-slate-600">Voided</span>}
    </div>
    <div className={`grid grid-cols-[1fr_auto_1fr] items-center gap-3 ${game.voided_at ? 'opacity-60' : ''}`}>
      <p className={`text-sm ${game.score_a > game.score_b ? 'font-bold text-primary-900' : 'text-slate-600'}`}>{names(game.team_a)}</p>
      <p className="text-2xl font-bold tabular-nums text-slate-950">{game.score_a} <span className="text-slate-400">:</span> {game.score_b}</p>
      <p className={`text-right text-sm ${game.score_b > game.score_a ? 'font-bold text-primary-900' : 'text-slate-600'}`}>{names(game.team_b)}</p>
    </div>
    <p className="text-xs text-slate-500">Recorded by {game.recorder_name}{game.version > 1 && ` · Updated by ${game.editor_name}`}</p>
    {editing ? <GameForm sessionId={game.session_id} editing={game} onSaved={change} onCancel={() => setEditing(false)} /> : <div className="flex flex-wrap gap-3">
      {canEdit && <button type="button" className="text-sm font-medium text-primary-700 min-h-10" onClick={() => setEditing(true)}>Correct result</button>}
      {canEdit && <button type="button" className="text-sm font-medium text-red-700 min-h-10" disabled={busy} onClick={() => setConfirmVoid(true)}>Void result</button>}
      <button type="button" className="text-sm font-medium text-slate-600 min-h-10" disabled={busy} onClick={() => void showHistory()}>{history ? 'Hide history' : 'History'}</button>
    </div>}
    {confirmVoid && <div className="rounded-xl bg-red-50 p-3 space-y-3">
      <p className="text-sm text-red-900">Void this result? It will stay in history and stop counting in comparisons.</p>
      <div className="flex gap-3"><button type="button" className="btn-primary" disabled={busy} onClick={() => void voidResult()}>Confirm void</button><button type="button" className="btn-outline" disabled={busy} onClick={() => setConfirmVoid(false)}>Keep result</button></div>
    </div>}
    {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
    {history && <ol className="border-t border-slate-100 pt-3 space-y-3" aria-label="Result history">{history.map((revision) => <li key={revision.version} className="text-sm text-slate-600">
      <p className="font-medium">Version {revision.version}{revision.voided_at ? ' · Voided' : ''}: {names(revision.team_a)} {revision.score_a} : {revision.score_b} {names(revision.team_b)}</p>
      <p className="text-xs">{revision.editor_name} · {formatInTimeZone(revision.updated_at, 'Australia/Sydney', 'd MMM yyyy, h:mm a')}</p>
    </li>)}</ol>}
  </article>;
}

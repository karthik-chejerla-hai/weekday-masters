import { useEffect, useId, useRef, useState } from 'react';
import { Check, Loader2 } from 'lucide-react';
import { api } from '../../services/api';
import { useApiMutation } from '../../hooks/useApi';
import { displayName } from '../../utils/members';
import { assistantError } from '../assistant/errors';
import type { GameDraft, GamePlayer, GameResult } from '../../types';

interface Props {
  sessionId: string;
  initial?: GameDraft;
  editing?: GameResult;
  onSaved: () => void;
  onCancel?: () => void;
}
export default function GameForm({ sessionId, initial, editing, onSaved, onCancel }: Props) {
  const start = editing ?? initial;
  const prefix = useId();
  const [players, setPlayers] = useState<GamePlayer[]>(() => [...(start?.team_a ?? []), ...(start?.team_b ?? [])]);
  const [teams, setTeams] = useState<string[]>(() => start ? [...start.team_a, ...start.team_b].map((p) => p.id) : ['', '', '', '']);
  const [scores, setScores] = useState<[string, string]>(() => start ? [String(start.score_a), String(start.score_b)] : ['', '']);
  const [error, setError] = useState('');
  const [memberError, setMemberError] = useState('');
  const [saved, setSaved] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const { mutate, isLoading } = useApiMutation<GameResult, []>();
  const lock = useRef(false);
  const request = useRef<{ signature: string; id: string } | null>(null);

  useEffect(() => {
    let active = true;
    api.listMembers().then((members) => {
      if (!active) return;
      const options = new Map([...(start?.team_a ?? []), ...(start?.team_b ?? [])].map((p) => [p.id, p]));
      members.forEach((m) => options.set(m.id, { id: m.id, name: displayName(m), full_name: m.name }));
      setPlayers([...options.values()].sort((a, b) => a.name.localeCompare(b.name)));
      setMemberError('');
    }).catch(() => { if (active) setMemberError('Could not load the member list. Try again.'); });
    return () => { active = false; };
  }, [start, loadAttempt]);

  const save = async () => {
    if (lock.current) return;
    setError(''); setSaved(false);
    if (teams.some((id) => !id) || new Set(teams).size !== 4) { setError('Choose four different players.'); return; }
    if (scores.some((score) => !/^\d{1,2}$/.test(score)) || Number(scores[0]) === Number(scores[1])) { setError('Enter two different whole-number scores from 0 to 99.'); return; }
    const input = { team_a: teams.slice(0, 2), team_b: teams.slice(2), score_a: Number(scores[0]), score_b: Number(scores[1]) };
    const signature = JSON.stringify(input);
    if (!request.current || request.current.signature !== signature) request.current = { signature, id: crypto.randomUUID() };
    const requestId = request.current.id;
    lock.current = true;
    try {
      await mutate(() => editing ? api.updateGame(editing.id, { ...input, version: editing.version }) : api.createGame(sessionId, { ...input, request_id: requestId }));
      request.current = null; setSaved(true);
      if (!editing) setScores(['', '']);
      onSaved();
    } catch (err) { setError(assistantError(err, 'Could not save the game. Your entry is still here. Try again.')); }
    finally { lock.current = false; }
  };

  return <form onSubmit={(event) => { event.preventDefault(); void save(); }} className="space-y-4" aria-label={editing ? 'Correct game' : 'Game result'}>
    {memberError && <div role="alert" className="text-sm text-red-700">{memberError} <button type="button" className="underline" onClick={() => setLoadAttempt((n) => n + 1)}>Retry members</button></div>}
    <fieldset disabled={isLoading} className="grid grid-cols-2 gap-3 sm:gap-4">
      {['A', 'B'].map((side, sideIndex) => <div key={side} className={`min-w-0 rounded-2xl border p-3 sm:p-4 space-y-3 ${side === 'A' ? 'border-primary-200 bg-primary-50/50' : 'border-secondary-200 bg-secondary-50/50'}`}>
        <h3 className="font-semibold text-slate-950">Team {side}</h3>
        {[0, 1].map((slot) => {
          const index = sideIndex * 2 + slot; const id = `${prefix}-player-${index}`;
          return <div key={slot}><label htmlFor={id} className="label">Player {slot + 1}</label>
            <select id={id} aria-label={`Team ${side} player ${slot + 1}`} className="input min-h-12" value={teams[index]} onChange={(event) => { setSaved(false); setTeams((old) => old.map((value, i) => i === index ? event.target.value : value)); }}>
              <option value="">Choose player</option>
              {players.map((p) => <option key={p.id} value={p.id}>{players.filter((candidate) => candidate.name === p.name).length > 1 && p.full_name ? `${p.name} (${p.full_name})` : p.name}</option>)}
            </select></div>;
        })}
        <div><label className="label" htmlFor={`${prefix}-score-${side}`}>Score</label>
          <input id={`${prefix}-score-${side}`} aria-label={`Team ${side} score`} className="input min-h-16 text-center text-3xl font-bold tabular-nums" type="number" inputMode="numeric" min="0" max="99" step="1" value={scores[sideIndex]} onChange={(event) => { setSaved(false); setScores((old) => sideIndex === 0 ? [event.target.value, old[1]] : [old[0], event.target.value]); }} />
        </div>
      </div>)}
    </fieldset>
    <p className="text-sm text-slate-600">One completed doubles game. Check the teams and their scores before saving.</p>
    {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
    {saved && <p role="status" className="flex items-center gap-2 text-sm text-emerald-700"><Check className="h-4 w-4" />Game saved. Enter the next score when ready.</p>}
    <div className="flex flex-wrap gap-3">
      <button type="submit" className="btn-primary min-h-12 flex-1 gap-2" disabled={isLoading}>{isLoading && <Loader2 className="h-4 w-4 animate-spin" />}{isLoading ? 'Saving…' : editing ? 'Save correction' : 'Save game'}</button>
      {onCancel && <button type="button" className="btn-outline" disabled={isLoading} onClick={onCancel}>Cancel</button>}
    </div>
  </form>;
}

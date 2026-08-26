import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { AlertTriangle, Loader2, Plus } from 'lucide-react';
import { api } from '../services/api';
import ParticipantRow, { type EditableLine } from '../components/settlement/ParticipantRow';
import SettlementBreakdown from '../components/settlement/SettlementBreakdown';
import { formatCents, parseDollarsToCents } from '../components/money/format';
import type { ApiError, PlayerBalance, SettlementInput, SettlementPreview } from '../types';

/** The shortfall the server reports when the bag cannot cover the night. */
interface StockShortfall {
  required_units: number;
  available_units: number;
}

function readApiError(err: unknown): ApiError | null {
  const response = (err as { response?: { data?: ApiError } })?.response;
  return response?.data?.code ? response.data : null;
}

export default function AdminSettlement() {
  const { id: sessionId } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [members, setMembers] = useState<PlayerBalance[]>([]);
  const [lines, setLines] = useState<EditableLine[]>([]);
  const [extraHours, setExtraHours] = useState(0);
  const [preview, setPreview] = useState<SettlementPreview | null>(null);

  const [isLoading, setIsLoading] = useState(true);
  // The seed request already returns a costed preview, so the first run of the
  // refresh effect would be an identical second request on open.
  const hasSeeded = useRef(false);
  const [isSettling, setIsSettling] = useState(false);
  const [shortfall, setShortfall] = useState<StockShortfall | null>(null);
  const [error, setError] = useState<string | null>(null);

  const hasExtraHour = extraHours > 0;

  const input = useMemo<SettlementInput>(
    () => ({
      extra_hours: extraHours,
      lines: lines.map(({ user_id, guest_name, in_base, in_extra, comped }) => ({
        user_id,
        guest_name,
        in_base,
        in_extra: hasExtraHour && in_extra,
        comped,
      })),
    }),
    [lines, extraHours, hasExtraHour]
  );

  // Seed the form from the session's RSVPs, which is what the server would
  // default to anyway.
  useEffect(() => {
    if (!sessionId) return;

    (async () => {
      try {
        const [balances, seeded] = await Promise.all([
          api.listBalances(),
          api.previewSettlement(sessionId, {}),
        ]);
        setMembers(balances);
        setLines(
          seeded.lines.map((line) => ({
            user_id: line.user_id,
            name: line.name ?? '',
            guest_name: line.guest_name,
            in_base: line.in_base,
            in_extra: line.in_extra,
            comped: line.comped,
          }))
        );
        setPreview(seeded);
        hasSeeded.current = true;
      } catch (err) {
        const apiError = readApiError(err);
        setError(apiError?.message ?? 'Could not open this session for settling.');
      } finally {
        setIsLoading(false);
      }
    })();
  }, [sessionId]);

  // Re-cost on every change, so what is on screen is what settling will post.
  const refreshPreview = useCallback(async () => {
    if (!sessionId || lines.length === 0) {
      setPreview(null);
      return;
    }

    try {
      const next = await api.previewSettlement(sessionId, input);
      setPreview(next);
      setShortfall(null);
      setError(null);
    } catch (err) {
      const apiError = readApiError(err);
      if (apiError?.code === 'shuttle_stock_short') {
        setShortfall(apiError.details as unknown as StockShortfall);
        setPreview(null);
        return;
      }
      setError(apiError?.message ?? 'Could not work out what this session cost.');
      setPreview(null);
    }
  }, [sessionId, input, lines.length]);

  useEffect(() => {
    if (isLoading) return;
    // Skip the run triggered by seeding; re-cost only once something changes.
    if (hasSeeded.current) {
      hasSeeded.current = false;
      return;
    }
    refreshPreview();
  }, [isLoading, refreshPreview]);

  const updateLine = (index: number, next: EditableLine) =>
    setLines((current) => current.map((line, i) => (i === index ? next : line)));

  const removeLine = (index: number) =>
    setLines((current) => current.filter((_, i) => i !== index));

  const addMember = (userId: string) => {
    const member = members.find((m) => m.user_id === userId);
    if (!member) return;
    setLines((current) => [
      ...current,
      {
        user_id: member.user_id,
        name: member.name,
        profile_picture: member.profile_picture,
        in_base: true,
        in_extra: hasExtraHour,
        comped: false,
      },
    ]);
  };

  const addGuest = (hostId: string, guestName: string) => {
    const host = members.find((m) => m.user_id === hostId);
    if (!host || !guestName.trim()) return;
    setLines((current) => [
      ...current,
      {
        user_id: host.user_id,
        name: host.name,
        guest_name: guestName.trim(),
        in_base: true,
        in_extra: hasExtraHour,
        comped: false,
      },
    ]);
  };

  const handleSettle = async () => {
    if (!sessionId || !preview) return;

    setIsSettling(true);
    setError(null);
    try {
      await api.settleSession(sessionId, input);
      navigate('/sessions');
    } catch (err) {
      const apiError = readApiError(err);
      if (apiError?.code === 'shuttle_stock_short') {
        setShortfall(apiError.details as unknown as StockShortfall);
      } else {
        setError(apiError?.message ?? 'Could not settle this session.');
      }
    } finally {
      setIsSettling(false);
    }
  };

  if (isLoading) {
    return (
      <div className="card p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-lg font-semibold text-slate-900">Settle this session</h1>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {shortfall && (
        <ShuttleShortfallPrompt
          shortfall={shortfall}
          onRecorded={async () => {
            setShortfall(null);
            await refreshPreview();
          }}
        />
      )}

      <div className="card p-4 space-y-3">
        <label className="label" htmlFor="extra-hours">Did you play on?</label>
        <select
          id="extra-hours"
          className="input"
          value={extraHours}
          onChange={(e) => {
            const hours = Number(e.target.value);
            setExtraHours(hours);
            // Ticking everyone in by default when an extension is added is the
            // common case; the admin unticks whoever went home.
            setLines((current) => current.map((line) => ({ ...line, in_extra: hours > 0 })));
          }}
        >
          <option value={0}>No — standard hours only</option>
          <option value={1}>Yes — one extra hour</option>
          <option value={2}>Yes — two extra hours</option>
        </select>
      </div>

      <div>
        <h2 className="text-sm font-semibold text-slate-900 mb-2">Who played</h2>
        <ul
          aria-label="Participants"
          className="rounded-xl border border-slate-200 bg-white divide-y divide-slate-100"
        >
          {lines.map((line, index) => (
            <ParticipantRow
              key={`${line.user_id}-${line.guest_name ?? ''}-${index}`}
              line={line}
              costed={preview?.lines[index]}
              hasExtraHour={hasExtraHour}
              onChange={(next) => updateLine(index, next)}
              onRemove={() => removeLine(index)}
            />
          ))}
          {lines.length === 0 && (
            <li className="px-4 py-6 text-center text-sm text-slate-500">
              Nobody on this session yet.
            </li>
          )}
        </ul>
      </div>

      <AddParticipant members={members} onAddMember={addMember} onAddGuest={addGuest} />

      {preview && (
        <div>
          <h2 className="text-sm font-semibold text-slate-900 mb-2">What it comes to</h2>
          <SettlementBreakdown preview={preview} />
        </div>
      )}

      <button
        type="button"
        className="btn-primary w-full"
        disabled={!preview || isSettling || lines.length === 0}
        onClick={handleSettle}
      >
        {isSettling
          ? 'Settling…'
          : preview
            ? `Settle — charge ${formatCents(preview.totals.charged_cents)}`
            : 'Settle'}
      </button>
    </div>
  );
}

/**
 * Offer to record the shuttles the admin forgot to enter, without leaving the
 * form. Settlement refuses to run stock negative, so this is the way forward
 * rather than a dead end.
 */
function ShuttleShortfallPrompt({
  shortfall,
  onRecorded,
}: {
  shortfall: StockShortfall;
  onRecorded: () => void;
}) {
  const [units, setUnits] = useState('12');
  const [cost, setCost] = useState('50.00');
  const [isSaving, setIsSaving] = useState(false);

  const handleRecord = async () => {
    const amountCents = parseDollarsToCents(cost);
    const count = Number.parseInt(units, 10);
    if (!Number.isFinite(amountCents) || !Number.isInteger(count) || count <= 0) return;

    setIsSaving(true);
    try {
      await api.recordShuttlePurchase(count, amountCents, 'Recorded while settling');
      onRecorded();
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="rounded-lg border border-secondary-300 bg-secondary-50 p-4 space-y-3">
      <div className="flex items-start gap-2">
        <AlertTriangle className="w-5 h-5 text-secondary-600 flex-shrink-0 mt-0.5" />
        <p className="text-sm text-secondary-900">
          This session uses {shortfall.required_units} shuttles but stock holds{' '}
          {shortfall.available_units}. Record the tube you have not entered yet and carry on.
        </p>
      </div>

      <div className="flex gap-2">
        <input
          className="input"
          type="number"
          min="1"
          value={units}
          onChange={(e) => setUnits(e.target.value)}
          aria-label="Shuttles bought"
        />
        <input
          className="input"
          type="number"
          step="0.01"
          min="0.01"
          value={cost}
          onChange={(e) => setCost(e.target.value)}
          aria-label="What they cost"
        />
        <button type="button" className="btn-secondary whitespace-nowrap" onClick={handleRecord} disabled={isSaving}>
          {isSaving ? 'Saving…' : 'Record'}
        </button>
      </div>
    </div>
  );
}

function AddParticipant({
  members,
  onAddMember,
  onAddGuest,
}: {
  members: PlayerBalance[];
  onAddMember: (userId: string) => void;
  onAddGuest: (hostId: string, guestName: string) => void;
}) {
  const [memberId, setMemberId] = useState('');
  const [hostId, setHostId] = useState('');
  const [guestName, setGuestName] = useState('');

  return (
    <div className="card p-4 space-y-4">
      <div>
        <label className="label" htmlFor="add-member">Add someone who turned up</label>
        <div className="flex gap-2">
          <select
            id="add-member"
            className="input"
            value={memberId}
            onChange={(e) => setMemberId(e.target.value)}
          >
            <option value="">Choose a member…</option>
            {members.map((m) => (
              <option key={m.user_id} value={m.user_id}>{m.name}</option>
            ))}
          </select>
          <button
            type="button"
            className="btn-outline whitespace-nowrap"
            disabled={!memberId}
            onClick={() => {
              onAddMember(memberId);
              setMemberId('');
            }}
          >
            <Plus className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div>
        <label className="label" htmlFor="guest-name">Add a guest</label>
        <div className="flex gap-2">
          <input
            id="guest-name"
            className="input"
            placeholder="Guest name"
            value={guestName}
            onChange={(e) => setGuestName(e.target.value)}
          />
          <select
            className="input"
            value={hostId}
            onChange={(e) => setHostId(e.target.value)}
            aria-label="Charged to"
          >
            <option value="">Charged to…</option>
            {members.map((m) => (
              <option key={m.user_id} value={m.user_id}>{m.name}</option>
            ))}
          </select>
          <button
            type="button"
            className="btn-outline whitespace-nowrap"
            disabled={!hostId || !guestName.trim()}
            onClick={() => {
              onAddGuest(hostId, guestName);
              setGuestName('');
            }}
          >
            <Plus className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

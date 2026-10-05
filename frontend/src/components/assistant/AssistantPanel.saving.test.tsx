import { beforeEach, expect, it, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AssistantPanel from './AssistantPanel';
import { api } from '../../services/api';
import type { GamePreview, GameResult } from '../../types';

const voice = vi.hoisted<{ onTranscript: (text: string) => void }>(() => ({ onTranscript: () => {} }));
vi.mock('../../services/api', () => ({ api: { assistantStatus: vi.fn(), askAssistant: vi.fn(), listMembers: vi.fn(), createGame: vi.fn() } }));
vi.mock('../../context/useAuth', () => ({ useAuth: () => ({ isAdmin: false }) }));
vi.mock('./VoiceRecorder', () => ({
  default: ({ onTranscript, disabled }: { onTranscript: (text: string) => void; disabled: boolean }) => {
    voice.onTranscript = onTranscript;
    return <button disabled={disabled}>Mock recording</button>;
  },
}));

const players = ['Alice', 'Bob', 'Cara', 'Dan'].map((name, i) => ({ id: String(i), name }));
const preview: GamePreview = {
  session: { id: 's1', title: 'Club night', starts_at: '', ends_at: '' },
  team_a: players.slice(0, 2), team_b: players.slice(2), score_a: 21, score_b: 17,
};
const nextRequest = 'Alice and Bob beat Cara and Dan, 21 to 19';

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.assistantStatus).mockResolvedValue({ enabled: true });
  vi.mocked(api.listMembers).mockResolvedValue(players as never);
  vi.mocked(api.askAssistant).mockResolvedValue({ message: 'Review the game.', game: preview });
});

async function openReview() {
  const user = userEvent.setup();
  render(<MemoryRouter><AssistantPanel sessionId="s1" mode="games" /></MemoryRouter>);
  await user.type(screen.getByLabelText('Your message'), 'Alice and Bob beat Cara and Dan, 21 to 17');
  await user.click(screen.getByRole('button', { name: 'Send message' }));
  await screen.findByRole('region', { name: 'Review game' });
  return user;
}

it('blocks new requests during a save and retains a late transcript for the next game', async () => {
  let completeSave!: (result: GameResult) => void;
  vi.mocked(api.createGame).mockReturnValue(new Promise((resolve) => { completeSave = resolve; }));
  const user = await openReview();
  // A recording retains the callback from the render in which it started.
  const finishRecording = voice.onTranscript;
  await user.click(screen.getByRole('button', { name: 'Save game' }));

  expect(screen.getByLabelText('Your message')).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Mock recording' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
  await user.type(screen.getByLabelText('Your message'), nextRequest);
  expect(screen.getByLabelText('Your message')).toHaveValue('');
  act(() => finishRecording(nextRequest));
  expect(screen.getByLabelText('Your message')).toHaveValue(nextRequest);
  expect(screen.getByLabelText('Team B score')).toHaveValue(17);
  expect(api.askAssistant).toHaveBeenCalledTimes(1);

  await act(async () => { completeSave({ id: 'first-game' } as GameResult); });
  expect(screen.getByText('Game saved.')).toBeInTheDocument();
  expect(screen.getByLabelText('Your message')).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Mock recording' })).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Send message' })).toBeEnabled();
  vi.mocked(api.askAssistant).mockResolvedValue({ message: 'Review the next game.', game: { ...preview, score_b: 19 } });
  await user.click(screen.getByRole('button', { name: 'Send message' }));
  expect(await screen.findByLabelText('Team B score')).toHaveValue(19);
  expect(api.createGame).toHaveBeenCalledTimes(1);
});

it('unlocks input after a failed save and keeps the reviewed game for an idempotent retry', async () => {
  let failSave!: (reason: Error) => void;
  vi.mocked(api.createGame).mockReturnValue(new Promise((_resolve, reject) => { failSave = reject; }));
  const user = await openReview();
  const finishRecording = voice.onTranscript;
  await user.click(screen.getByRole('button', { name: 'Save game' }));
  act(() => finishRecording(nextRequest));
  await act(async () => { failSave(new Error('network')); });

  expect(screen.getByRole('alert')).toHaveTextContent('Could not save the game');
  expect(screen.getByLabelText('Team B score')).toHaveValue(17);
  expect(screen.getByLabelText('Your message')).toBeEnabled();
  expect(screen.getByLabelText('Your message')).toHaveValue(nextRequest);
  expect(screen.getByRole('button', { name: 'Mock recording' })).toBeEnabled();
  expect(screen.queryByText('Game saved.')).not.toBeInTheDocument();
  vi.mocked(api.createGame).mockResolvedValue({ id: 'first-game' } as GameResult);
  await user.click(screen.getByRole('button', { name: 'Save game' }));
  expect(await screen.findByText('Game saved.')).toBeInTheDocument();
  const calls = vi.mocked(api.createGame).mock.calls;
  expect(calls).toHaveLength(2);
  expect(calls[0][1].request_id).toBeTruthy();
  expect(calls[1][1]).toEqual(calls[0][1]);
});

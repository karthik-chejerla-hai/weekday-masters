import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AssistantPanel from './AssistantPanel';
import { api } from '../../services/api';
import { expenseFixture } from './fixtures';
import { useAuth } from '../../context/useAuth';
vi.mock('../../services/api', () => ({ api: { assistantStatus: vi.fn(), askAssistant: vi.fn(), confirmExpense: vi.fn(), listMembers: vi.fn(), createGame: vi.fn() } }));
vi.mock('../../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('./VoiceRecorder', () => ({ default: ({ onTranscript }: { onTranscript: (text: string) => void }) => <button onClick={() => onTranscript('Three hours and eight shuttles')}>Mock recording</button> }));
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.assistantStatus).mockResolvedValue({ enabled: true });
  vi.mocked(useAuth).mockReturnValue({ isAdmin: true } as never);
});
it('lets the user review the transcript before requesting a preview', async () => {
  vi.mocked(api.askAssistant).mockResolvedValue({ message: 'Review this expense.', expense: expenseFixture() });
  const user = userEvent.setup(); render(<MemoryRouter><AssistantPanel sessionId="s1" /></MemoryRouter>);
  await user.click(screen.getByRole('button', { name: 'Mock recording' }));
  expect(screen.getByLabelText('Your message')).toHaveValue('Three hours and eight shuttles');
  expect(api.askAssistant).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Send message' }));
  expect(await screen.findByRole('region', { name: 'Expense preview' })).toBeInTheDocument();
  expect(api.askAssistant).toHaveBeenCalledWith([{ role: 'user', content: 'Three hours and eight shuttles' }], 's1', expect.any(AbortSignal));
  expect(api.confirmExpense).not.toHaveBeenCalled();
});
it('keeps follow-up questions in the next request', async () => {
  vi.mocked(api.askAssistant).mockResolvedValueOnce({ message: 'How many shuttles?' }).mockResolvedValueOnce({ message: 'Review this expense.', expense: expenseFixture() });
  const user = userEvent.setup(); render(<MemoryRouter><AssistantPanel sessionId="s1" /></MemoryRouter>);
  await user.type(screen.getByLabelText('Your message'), 'Three hours'); await user.click(screen.getByRole('button', { name: 'Send message' }));
  await screen.findByText('How many shuttles?');
  await user.type(screen.getByLabelText('Your message'), 'Eight'); await user.click(screen.getByRole('button', { name: 'Send message' }));
  await screen.findByRole('region', { name: 'Expense preview' });
  expect(vi.mocked(api.askAssistant).mock.calls[1][0]).toEqual([{ role: 'user', content: 'Three hours' }, { role: 'assistant', content: 'How many shuttles?' }, { role: 'user', content: 'Eight' }]);
});
it('keeps the form available without a key', async () => {
  vi.mocked(api.assistantStatus).mockResolvedValue({ enabled: false });
  render(<MemoryRouter><AssistantPanel sessionId="s1" /></MemoryRouter>);
  expect(await screen.findByRole('status')).toHaveTextContent('unavailable');
  expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
  expect(screen.getByRole('link', { name: 'Use the expense form' })).toHaveAttribute('href', '/admin/sessions/s1/expense');
});
it('shows a quota error and preserves the request for retry', async () => {
  vi.mocked(api.askAssistant).mockRejectedValue({ response: { data: { message: 'Usage limit reached.' } } });
  const user = userEvent.setup(); render(<MemoryRouter><AssistantPanel /></MemoryRouter>);
  await user.type(screen.getByLabelText('Your message'), 'How much court credit?'); await user.click(screen.getByRole('button', { name: 'Send message' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Usage limit reached.');
  expect(screen.getByLabelText('Your message')).toHaveValue('How much court credit?');
});

it('turns a game recording into a review and saves only on explicit confirmation', async () => {
  const players = ['Alice', 'Bob', 'Cara', 'Dan'].map((name, i) => ({ id: String(i), name }));
  vi.mocked(api.listMembers).mockResolvedValue(players as never);
  vi.mocked(api.createGame).mockResolvedValue({id: 'game'} as never);
  vi.mocked(api.askAssistant).mockResolvedValue({ message: 'Review game', game: { session: {id:'s1', title:'Club night', starts_at:'', ends_at:''}, team_a:players.slice(0,2), team_b:players.slice(2), score_a:21, score_b:17 } });
  vi.mocked(useAuth).mockReturnValue({ isAdmin: false } as never);
  const user = userEvent.setup(); const saved = vi.fn();
  render(<MemoryRouter><AssistantPanel sessionId="s1" mode="games" onGameSaved={saved} /></MemoryRouter>);
  await user.click(screen.getByRole('button', { name: 'Mock recording' }));
  expect(await screen.findByRole('region', {name: 'Review game'})).toBeInTheDocument();
  expect(api.createGame).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', {name:'Save game'}));
  expect(await screen.findByText('Game saved.')).toBeInTheDocument();
  expect(saved).toHaveBeenCalledOnce();
});

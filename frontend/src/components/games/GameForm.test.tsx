import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, beforeEach, it, expect } from 'vitest';
import GameForm from './GameForm';
import { api } from '../../services/api';
vi.mock('../../services/api', () => ({ api: { listMembers: vi.fn(), createGame: vi.fn(), updateGame: vi.fn() } }));
const players = ['Alice', 'Bob', 'Cara', 'Dan'].map((name, i) => ({ id: `${i}`, name, nickname: '' }));
beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.listMembers).mockResolvedValue(players as never); });
it('records a manual doubles result and keeps the request ID on retry', async () => {
 const user = userEvent.setup(); const saved = vi.fn();
 vi.mocked(api.createGame).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ id: 'game' } as never);
 render(<GameForm sessionId="session" onSaved={saved} />);
 for (const [i, label] of ['Team A player 1', 'Team A player 2', 'Team B player 1', 'Team B player 2'].entries()) { await user.selectOptions(await screen.findByLabelText(label), `${i}`); }
 await user.type(screen.getByLabelText('Team A score'), '21'); await user.type(screen.getByLabelText('Team B score'), '17');
 await user.click(screen.getByRole('button', { name: 'Save game' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('Could not save');
 await user.click(screen.getByRole('button', { name: 'Save game' }));
 await waitFor(() => expect(saved).toHaveBeenCalledOnce());
 const calls = vi.mocked(api.createGame).mock.calls;
 expect(calls[0][1]).toEqual(calls[1][1]); expect(calls[0][1].request_id).toBeTruthy();
 expect(calls[0][1]).toMatchObject({ team_a: ['0', '1'], team_b: ['2', '3'], score_a: 21, score_b: 17 });
});
it('shows a voice preview without saving until the member presses Save', async () => {
 const user = userEvent.setup(); vi.mocked(api.createGame).mockResolvedValue({id: 'g'} as never);
 render(<GameForm sessionId="s" initial={{team_a: players.slice(0, 2), team_b: players.slice(2), score_a: 21, score_b: 17}} onSaved={vi.fn()} />);
 expect(screen.getByLabelText('Team A score')).toHaveValue(21); expect(api.createGame).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button', {name: 'Save game'})); await waitFor(() => expect(api.createGame).toHaveBeenCalledOnce());
});
it('rejects duplicate players and equal scores', async () => {
 const user = userEvent.setup();
 render(<GameForm sessionId="s" initial={{team_a: players.slice(0, 2), team_b: [players[0], players[3]], score_a: 21, score_b: 21}} onSaved={vi.fn()} />);
 await user.click(screen.getByRole('button', {name:'Save game'})); expect(await screen.findByRole('alert')).toHaveTextContent('four different'); expect(api.createGame).not.toHaveBeenCalled();
});

it('uses full names to identify members with the same display name', async () => {
 vi.mocked(api.listMembers).mockResolvedValue([{id:'one',name:'Alex One',nickname:''},{id:'two',name:'Alex Two',nickname:''}] as never);
 render(<GameForm sessionId="s" onSaved={vi.fn()} />);
 expect((await screen.findAllByRole('option', {name:'Alex (Alex One)'})).length).toBe(4);
 expect(screen.getAllByRole('option', {name:'Alex (Alex Two)'}).length).toBe(4);
});

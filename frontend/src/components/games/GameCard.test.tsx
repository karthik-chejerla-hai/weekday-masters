import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { vi, it, expect } from 'vitest';
import GameCard from './GameCard';
import { api } from '../../services/api';
import type { GameResult } from '../../types';
vi.mock('../../services/api', () => ({api:{voidGame:vi.fn(),gameRevisions:vi.fn(),listMembers:vi.fn()}}));
vi.mock('../../context/useAuth', () => ({useAuth:() => ({user:{id:'recorder'},isAdmin:false})}));
const game = {id:'g',session_id:'s',session_title:'Club night',session_date:'2026-10-01',team_a:[{id:'a',name:'Alice'},{id:'b',name:'Bob'}],team_b:[{id:'c',name:'Cara'},{id:'d',name:'Dan'}],score_a:21,score_b:17,version:1,created_by:'recorder',updated_by:'recorder',recorder_name:'Eve',editor_name:'Eve',created_at:'2026-10-01T09:00:00Z',updated_at:'2026-10-01T09:00:00Z',voided_at:null} satisfies GameResult;
it('requires a second action to void and shows revision history',async()=>{
 const user=userEvent.setup(); const changed=vi.fn(); vi.mocked(api.voidGame).mockResolvedValue({...game,voided_at:'now'});vi.mocked(api.gameRevisions).mockResolvedValue([game]);
 render(<MemoryRouter><GameCard game={game} onChanged={changed}/></MemoryRouter>);
 await user.click(screen.getByRole('button',{name:'History'}));expect(await screen.findByText(/Version 1/)).toBeInTheDocument();
 await user.click(screen.getByRole('button',{name:'Void result'}));expect(api.voidGame).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button',{name:'Confirm void'}));await waitFor(()=>expect(changed).toHaveBeenCalledOnce());expect(api.voidGame).toHaveBeenCalledWith('g',1);
});
it('hides corrections for another recorder',()=>{
 render(<MemoryRouter><GameCard game={{...game,created_by:'other'}} onChanged={vi.fn()}/></MemoryRouter>);
 expect(screen.queryByRole('button',{name:'Correct result'})).not.toBeInTheDocument();
});

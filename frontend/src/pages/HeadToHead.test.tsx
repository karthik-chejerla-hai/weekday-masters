import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { vi, it, expect } from 'vitest';
import HeadToHead from './HeadToHead';
import { api } from '../services/api';
vi.mock('../services/api',()=>({api:{gamePlayers:vi.fn(),headToHead:vi.fn()}}));
vi.mock('../context/useAuth',()=>({useAuth:()=>({user:{id:'a'}})}));
it('compares players and exact teams without counting an empty record as a win',async()=>{
 const user=userEvent.setup();vi.mocked(api.gamePlayers).mockResolvedValue([{id:'a',name:'Alice'},{id:'b',name:'Bob'},{id:'c',name:'Cara'},{id:'d',name:'Dan'}]);
 vi.mocked(api.headToHead).mockResolvedValue({items:[],total:0,wins_a:0,wins_b:0,points_a:0,points_b:0});
 render(<MemoryRouter><HeadToHead/></MemoryRouter>);
 await user.selectOptions(await screen.findByLabelText('Side B player 1'),'b');await user.click(screen.getByRole('button',{name:'Show record'}));
 expect(await screen.findByText('No games between these opponents yet.')).toBeInTheDocument();
 await user.click(screen.getByRole('button',{name:'Exact teams'}));
 await user.selectOptions(screen.getByLabelText('Side A player 2'),'c');await user.selectOptions(screen.getByLabelText('Side B player 2'),'d');
 await user.click(screen.getByRole('button',{name:'Show record'}));
 await waitFor(()=>expect(api.headToHead).toHaveBeenLastCalledWith(['a','c'],['b','d'],0,expect.any(AbortSignal)));
});

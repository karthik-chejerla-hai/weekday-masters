import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { beforeEach, it, expect, vi } from 'vitest';
import SessionGames from './SessionGames';
import { api } from '../services/api';
vi.mock('../services/api', () => ({api:{getSession:vi.fn(),listGames:vi.fn(),assistantStatus:vi.fn(),listMembers:vi.fn()}}));
vi.mock('../context/useAuth', () => ({useAuth:()=>({user:{id:'me'},isAdmin:false})}));
beforeEach(()=>{vi.clearAllMocks();vi.mocked(api.listGames).mockResolvedValue({items:[],total:0});vi.mocked(api.assistantStatus).mockResolvedValue({enabled:false});vi.mocked(api.listMembers).mockResolvedValue([])});
const renderPage=()=>render(<MemoryRouter initialEntries={['/sessions/s/games']}><Routes><Route path="/sessions/:id/games" element={<SessionGames/>}/></Routes></MemoryRouter>);
it('keeps manual entry available when speech is unavailable',async()=>{
 vi.mocked(api.getSession).mockResolvedValue({session:{id:'s',title:'Club night',status:'open',starts_at:'2020-01-01T00:00:00Z'}} as never);renderPage();
 expect(await screen.findByText('Enter a score by hand')).toBeInTheDocument();
 expect(await screen.findByText(/Use the score form below/)).toBeInTheDocument();
 expect(screen.getByRole('link',{name:'Head-to-head'})).toHaveAttribute('href','/games');
});
it('shows history but prevents new entry for a cancelled session',async()=>{
 vi.mocked(api.getSession).mockResolvedValue({session:{id:'s',title:'Cancelled',status:'cancelled',starts_at:'2020-01-01T00:00:00Z'}} as never);renderPage();
 expect(await screen.findByText(/This session is cancelled/)).toBeInTheDocument();
 expect(screen.queryByText('Enter a score by hand')).not.toBeInTheDocument();
});
it('reports a failed load with a retry control',async()=>{
 vi.mocked(api.getSession).mockRejectedValue(new Error('offline'));renderPage();
 expect(await screen.findByRole('alert')).toHaveTextContent('Could not load');
 expect(screen.getByRole('button',{name:'Retry'})).toBeInTheDocument();
});

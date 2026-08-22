import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Home from './Home';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({ api: { getClub: vi.fn() } }));

const login = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAuth).mockReturnValue({ login } as unknown as ReturnType<typeof useAuth>);
  vi.mocked(api.getClub).mockResolvedValue({} as never);
});

describe('Home page', () => {
  it('offers the sign-in action', async () => {
    render(<Home />);

    await userEvent.click(screen.getByRole('button', { name: /sign in with google/i }));
    expect(login).toHaveBeenCalledOnce();
  });

  it('falls back to a default name before the club loads', async () => {
    render(<Home />);
    expect(await screen.findByRole('heading', { name: 'Rally', level: 1 })).toBeInTheDocument();
  });

  it('shows the club name and venue once loaded', async () => {
    vi.mocked(api.getClub).mockResolvedValue({
      name: 'Weekday Masters',
      venue_name: 'Olympic Park',
      venue_address: '1 Olympic Blvd',
    } as never);

    render(<Home />);

    expect(await screen.findByRole('heading', { name: 'Weekday Masters', level: 1 })).toBeInTheDocument();
    expect(screen.getByText('Olympic Park')).toBeInTheDocument();
    expect(screen.getByText('1 Olympic Blvd')).toBeInTheDocument();
  });

  it('still renders the landing page when the club request fails', async () => {
    vi.mocked(api.getClub).mockRejectedValue(new Error('offline'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    render(<Home />);

    expect(await screen.findByRole('button', { name: /sign in with google/i })).toBeInTheDocument();
    expect(screen.queryByText('Our Venue')).not.toBeInTheDocument();
  });
});

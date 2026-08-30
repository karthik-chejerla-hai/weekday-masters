import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Layout from './Layout';
import Header from './Header';
import Navigation from './Navigation';
import { useAuth } from '../../context/useAuth';

vi.mock('../../context/useAuth', () => ({ useAuth: vi.fn() }));

const logout = vi.fn();

function mockAuth(overrides: Record<string, unknown> = {}) {
  vi.mocked(useAuth).mockReturnValue({
    user: { name: 'Jane Player', profile_picture: 'https://pic.example/j.png' },
    isAdmin: false,
    logout,
    login: vi.fn(),
    isAuthenticated: true,
    isApproved: true,
    isLoading: false,
    refreshUser: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useAuth>);
}

function renderAt(ui: React.ReactNode, path = '/dashboard') {
  return render(<MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth();
});

describe('Header', () => {
  it('shows the signed-in member', () => {
    renderAt(<Header />);
    expect(screen.getByText('Jane')).toBeInTheDocument();
  });

  it('hides the admin link from ordinary members', () => {
    renderAt(<Header />);
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument();
  });

  it('shows the admin link to admins', () => {
    mockAuth({ isAdmin: true });
    renderAt(<Header />);
    expect(screen.getByRole('link', { name: 'Admin' })).toBeInTheDocument();
  });

  it('logs out when the logout control is used', async () => {
    renderAt(<Header />);
    await userEvent.click(screen.getByTitle('Logout'));
    expect(logout).toHaveBeenCalledOnce();
  });
});

describe('Navigation', () => {
  it('offers the member tabs', () => {
    renderAt(<Navigation />);
    for (const label of ['Home', 'Sessions', 'Profile']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.queryByText('Admin')).not.toBeInTheDocument();
  });

  it('adds an admin tab for admins', () => {
    mockAuth({ isAdmin: true });
    renderAt(<Navigation />);
    expect(screen.getByText('Admin')).toBeInTheDocument();
  });

  it('marks the current tab as active', () => {
    renderAt(<Navigation />, '/sessions');
    expect(screen.getByText('Sessions').closest('a')).toHaveClass('text-primary-600');
  });
});

describe('Layout', () => {
  it('renders the routed page between the header and the nav', () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/dashboard" element={<p>Routed content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Routed content')).toBeInTheDocument();
    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.getByRole('navigation')).toBeInTheDocument();
  });
});

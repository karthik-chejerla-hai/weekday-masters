import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthProvider } from './AuthContext';
import { useAuth } from './useAuth';
import { useAuth0 } from '@auth0/auth0-react';
import { api } from '../services/api';

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: vi.fn(),
}));

vi.mock('../services/api', () => ({
  api: {
    setAccessToken: vi.fn(),
    authCallback: vi.fn(),
    getMe: vi.fn(),
  },
}));

function TestConsumer() {
  const { user, isAuthenticated, isApproved, isAdmin, isLoading } = useAuth();
  if (isLoading) return <div>Loading Auth...</div>;
  return (
    <div>
      <div>Authenticated: {isAuthenticated ? 'Yes' : 'No'}</div>
      <div>Approved: {isApproved ? 'Yes' : 'No'}</div>
      <div>Admin: {isAdmin ? 'Yes' : 'No'}</div>
      <div>User: {user?.name || 'None'}</div>
    </div>
  );
}

describe('AuthContext', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('handles unauthenticated state', async () => {
    vi.mocked(useAuth0).mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
      user: undefined,
      loginWithRedirect: vi.fn(),
      logout: vi.fn(),
      getAccessTokenSilently: vi.fn(),
    } as unknown as ReturnType<typeof useAuth0>);

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Authenticated: No')).toBeInTheDocument();
      expect(screen.getByText('User: None')).toBeInTheDocument();
    });
  });

  it('syncs user on authenticated login', async () => {
    vi.mocked(useAuth0).mockReturnValue({
      isAuthenticated: true,
      isLoading: false,
      user: { name: 'Jane Admin', picture: 'https://pic.jpg' },
      loginWithRedirect: vi.fn(),
      logout: vi.fn(),
      getAccessTokenSilently: vi.fn().mockResolvedValue('token-123'),
    } as unknown as ReturnType<typeof useAuth0>);

    vi.mocked(api.authCallback).mockResolvedValue({
      user: {
        id: 'u-1',
        auth0_id: 'auth0|1',
        email: 'jane@admin.com',
        name: 'Jane Admin',
        profile_picture: 'https://pic.jpg',
        phone_number: '',
        role: 'admin',
        is_player: true,
        membership_status: 'approved',
        created_at: '',
        updated_at: '',
      },
      is_new: false,
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Authenticated: Yes')).toBeInTheDocument();
      expect(screen.getByText('Approved: Yes')).toBeInTheDocument();
      expect(screen.getByText('Admin: Yes')).toBeInTheDocument();
      expect(screen.getByText('User: Jane Admin')).toBeInTheDocument();
    });
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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
  const {
    user,
    isAuthenticated,
    isApproved,
    isAdmin,
    isLoading,
    isViewingAsMember,
    startMemberPreview,
    stopMemberPreview,
    loginForInvitation,
    login,
    retryAuth,
    authError,
  } = useAuth();
  if (isLoading) return <div>Loading Auth...</div>;
  return (
    <div>
      {authError && <div role="alert">{authError}</div>}
      <button onClick={login}>Sign in</button>
      <button onClick={retryAuth}>Retry profile</button>
      <div>Authenticated: {isAuthenticated ? 'Yes' : 'No'}</div>
      <div>Approved: {isApproved ? 'Yes' : 'No'}</div>
      <div>Admin: {isAdmin ? 'Yes' : 'No'}</div>
      <div>Member preview: {isViewingAsMember ? 'Yes' : 'No'}</div>
      <div>User: {user?.name || 'None'}</div>
      <button onClick={startMemberPreview}>Preview as member</button>
      <button onClick={stopMemberPreview}>Exit preview</button>
      <button onClick={loginForInvitation}>Invitation sign-in</button>
    </div>
  );
}

describe('AuthContext', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns invitation sign-in to the welcome page and requests account selection', async () => {
    const loginWithRedirect = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useAuth0).mockReturnValue({
      isAuthenticated: false, isLoading: false, user: undefined,
      loginWithRedirect, logout: vi.fn(), getAccessTokenSilently: vi.fn(),
    } as unknown as ReturnType<typeof useAuth0>);
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    await userEvent.click(await screen.findByRole('button', { name: 'Invitation sign-in' }));
    expect(loginWithRedirect).toHaveBeenCalledWith({
      appState: { returnTo: '/welcome' },
      authorizationParams: { connection: 'google-oauth2', prompt: 'select_account' },
    });
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
        nickname: '',
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

  it('lets an admin preview the effective member role and exit again', async () => {
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
        nickname: '',
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

    const user = userEvent.setup();
    render(<AuthProvider><TestConsumer /></AuthProvider>);

    await waitFor(() => expect(screen.getByText('Admin: Yes')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: 'Preview as member' }));
    expect(screen.getByText('Admin: No')).toBeInTheDocument();
    expect(screen.getByText('Member preview: Yes')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Exit preview' }));
    expect(screen.getByText('Admin: Yes')).toBeInTheDocument();
    expect(screen.getByText('Member preview: No')).toBeInTheDocument();
  });
});


describe('session renewal failures', () => {
  beforeEach(() => vi.resetAllMocks());

  function mockSession(error: unknown, token?: string) {
    const loginWithRedirect = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useAuth0).mockReturnValue({
      isAuthenticated: true, isLoading: false, user: { name: 'Alex' },
      getAccessTokenSilently: token ? vi.fn().mockResolvedValue(token) : vi.fn().mockRejectedValue(error),
      loginWithRedirect, logout: vi.fn(),
    } as unknown as ReturnType<typeof useAuth0>);
    return loginWithRedirect;
  }

  it.each(['login_required', 'consent_required', 'interaction_required', 'missing_refresh_token', 'invalid_grant'])(
    'shows normal sign-in when silent renewal returns %s', async (code) => {
      const login = mockSession({ error: code });
      render(<AuthProvider><TestConsumer /></AuthProvider>);
      await screen.findByText('Authenticated: No');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(api.authCallback).not.toHaveBeenCalled();
      expect(api.setAccessToken).toHaveBeenCalledWith(null);
      await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
      expect(login).toHaveBeenCalledWith({ authorizationParams: { connection: 'google-oauth2' } });
    }
  );

  it('keeps unexpected renewal failures visible and clears them for a new sign-in', async () => {
    mockSession({ error: 'network_error' });
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    expect(await screen.findByRole('alert')).toHaveTextContent('We could not finish signing you in');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not hide a backend failure with an error code that also means renewal expired', async () => {
    mockSession(null, 'valid-token');
    vi.mocked(api.authCallback).mockRejectedValue({ error: 'login_required', response: { data: { error: 'Profile service unavailable.' } } });
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    expect(await screen.findByRole('alert')).toHaveTextContent('Profile service unavailable.');
  });

  it('reports a failure to start interactive sign-in', async () => {
    const login = mockSession({ error: 'missing_refresh_token' });
    login.mockRejectedValue(new Error('redirect failed'));
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    await screen.findByText('Authenticated: No');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Google sign-in could not start.');
  });
});


it('keeps Auth0 redirect failures visible even when their code requires sign-in', async () => {
  vi.mocked(useAuth0).mockReturnValue({
    isAuthenticated: false, isLoading: false, user: undefined,
    error: { error: 'login_required' }, getAccessTokenSilently: vi.fn(),
    loginWithRedirect: vi.fn(), logout: vi.fn(),
  } as unknown as ReturnType<typeof useAuth0>);
  render(<AuthProvider><TestConsumer /></AuthProvider>);
  expect(await screen.findByRole('alert')).toHaveTextContent('Google sign-in did not finish');
});

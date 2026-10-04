import { useCallback, useEffect, useState, ReactNode } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { api } from '../services/api';
import { AuthContext, type AuthContextType } from './auth-context';
import type { User } from '../types';

export function AuthProvider({ children }: { children: ReactNode }) {
  const {
    isAuthenticated: auth0IsAuthenticated,
    isLoading: auth0IsLoading,
    user: auth0User,
    loginWithRedirect,
    logout: auth0Logout,
    getAccessTokenSilently,
    error: auth0Error,
  } = useAuth0();

  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isViewingAsMember, setIsViewingAsMember] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);

  const syncUser = useCallback(async () => {
    setAuthError(null);
    if (!auth0IsAuthenticated || !auth0User) {
      setUser(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      const token = await getAccessTokenSilently();
      api.setAccessToken(token);

      // Sync user with backend
      const response = await api.authCallback(
        auth0User.name || '',
        auth0User.picture || ''
      );

      setUser(response.user);
    } catch (error) {
      console.error('Failed to sync user:', error);
      setAuthError((error as { response?: { data?: { error?: string } } })?.response?.data?.error
        || 'We could not finish signing you in. Try again or contact your club admin.');
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, [auth0IsAuthenticated, auth0User, getAccessTokenSilently]);

  const refreshUser = async () => {
    if (!auth0IsAuthenticated) return;
    try {
      const token = await getAccessTokenSilently();
      api.setAccessToken(token);
      const currentUser = await api.getMe();
      setUser(currentUser);
    } catch (error) {
      console.error('Failed to refresh user:', error);
    }
  };

  useEffect(() => {
    if (!auth0IsLoading) {
      syncUser();
    }
  }, [auth0IsLoading, syncUser]);

  const login = () => {
    loginWithRedirect({
      authorizationParams: {
        connection: 'google-oauth2',
      },
    });
  };

  const loginForInvitation = () => {
    loginWithRedirect({
      appState: { returnTo: '/welcome' },
      authorizationParams: { connection: 'google-oauth2', prompt: 'select_account' },
    }).catch(() => setAuthError('Google sign-in could not start. Please try again.'));
  };

  const logout = () => {
    api.setAccessToken(null);
    setUser(null);
    setIsViewingAsMember(false);
    auth0Logout({
      logoutParams: {
        returnTo: window.location.origin,
      },
    });
  };

  // This changes presentation only. The server still authorizes every request
  // from the real JWT, so preview mode cannot grant or revoke permissions.
  const startMemberPreview = () => {
    if (user?.role === 'admin') setIsViewingAsMember(true);
  };

  const stopMemberPreview = () => setIsViewingAsMember(false);

  const isAdminAccount = user?.role === 'admin';

  const value: AuthContextType = {
    user,
    isLoading: auth0IsLoading || isLoading,
    isAuthenticated: auth0IsAuthenticated && !!user,
    isApproved: user?.membership_status === 'approved',
    isAdmin: isAdminAccount && !isViewingAsMember,
    isViewingAsMember: isAdminAccount && isViewingAsMember,
    startMemberPreview,
    stopMemberPreview,
    login,
    loginForInvitation,
    authError: authError || (auth0Error ? 'Google sign-in did not finish. Try again or contact your club admin.' : null),
    retryAuth: syncUser,
    logout,
    refreshUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

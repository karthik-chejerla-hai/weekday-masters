import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { api } from '../services/api';
import type { User } from '../types';

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isApproved: boolean;
  isAdmin: boolean;
  login: () => void;
  logout: () => void;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const {
    isAuthenticated: auth0IsAuthenticated,
    isLoading: auth0IsLoading,
    user: auth0User,
    loginWithRedirect,
    logout: auth0Logout,
    getAccessTokenSilently,
  } = useAuth0();

  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const syncUser = async () => {
    if (!auth0IsAuthenticated || !auth0User) {
      setUser(null);
      setIsLoading(false);
      return;
    }

    try {
      const token = await getAccessTokenSilently();
      api.setAccessToken(token);

      // Sync user with backend
      const response = await api.authCallback(
        auth0User.sub || '',
        auth0User.email || '',
        auth0User.name || '',
        auth0User.picture || ''
      );

      setUser(response.user);
    } catch (error) {
      console.error('Failed to sync user:', error);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

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
  }, [auth0IsLoading, auth0IsAuthenticated, auth0User]);

  const login = () => {
    loginWithRedirect({
      authorizationParams: {
        connection: 'google-oauth2',
      },
    });
  };

  const logout = () => {
    api.setAccessToken(null);
    setUser(null);
    auth0Logout({
      logoutParams: {
        returnTo: window.location.origin,
      },
    });
  };

  const value: AuthContextType = {
    user,
    isLoading: auth0IsLoading || isLoading,
    isAuthenticated: auth0IsAuthenticated && !!user,
    isApproved: user?.membership_status === 'approved',
    isAdmin: user?.role === 'admin',
    login,
    logout,
    refreshUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

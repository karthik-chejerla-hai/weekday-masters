import type { ReactNode } from 'react';

// Only resolved by vite.e2e.config.ts. Keep function/user identities stable so
// AuthProvider's effect behaves like the real SDK rather than looping.
const user = { sub: 'auth0:browser-test', name: 'Alex Test', picture: '' };
const getAccessTokenSilently = async () => 'browser-test-token';
const loginWithRedirect = async (options: unknown) => {
  sessionStorage.setItem('e2e:login', JSON.stringify(options));
};
const logout = () => {
  sessionStorage.removeItem('e2e:authenticated');
  window.location.assign('/');
};

export function Auth0Provider({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth0() {
  const isAuthenticated = sessionStorage.getItem('e2e:authenticated') === 'true';
  return { isAuthenticated, isLoading: false, user: isAuthenticated ? user : undefined,
    getAccessTokenSilently, loginWithRedirect, logout, error: undefined };
}

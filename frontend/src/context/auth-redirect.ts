import { safeNotificationPath } from '../services/notification-destination';
export function safeAuthReturn(path: unknown): string {
  if (path === '/welcome' || path === '/dashboard' || path === '/sessions' || path === '/profile') return path;
  return safeNotificationPath(path) || '/';
}
// Auth0 changes browser history outside React Router: notify both.
export function handleAuthRedirect(appState?: { returnTo?: string }): void {
  const destination = safeAuthReturn(appState?.returnTo);
  window.history.replaceState({}, document.title, destination);
  window.dispatchEvent(new PopStateEvent('popstate'));
}

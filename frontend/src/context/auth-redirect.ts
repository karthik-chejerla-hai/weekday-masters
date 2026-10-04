// Auth0 changes browser history outside React Router. Notify the router as well
// so an invitation callback does not retain the old root-route redirect.
export function handleAuthRedirect(appState?: { returnTo?: string }): void {
  const destination = appState?.returnTo === '/welcome' ? '/welcome' : '/';
  window.history.replaceState({}, document.title, destination);
  window.dispatchEvent(new PopStateEvent('popstate'));
}

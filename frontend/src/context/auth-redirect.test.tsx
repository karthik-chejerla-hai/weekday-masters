import { afterEach, expect, it } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { handleAuthRedirect } from './auth-redirect';

afterEach(() => window.history.replaceState({}, '', '/'));
it('updates React Router after an invitation OAuth callback', () => {
  window.history.replaceState({}, '', '/?code=example');
  render(<BrowserRouter><Routes><Route path="/" element={<p>Home route</p>} /><Route path="/welcome" element={<p>Invitation result</p>} /></Routes></BrowserRouter>);
  act(() => handleAuthRedirect({ returnTo: '/welcome' }));
  expect(screen.getByText('Invitation result')).toBeInTheDocument();
  expect(window.location.search).toBe('');
});
it('rejects arbitrary redirect destinations', () => {
  handleAuthRedirect({ returnTo: 'https://outside.example' });
  expect(window.location.pathname).toBe('/');
});

it.each(['/sessions/11111111-1111-4111-8111-111111111111/settlement', '/notifications/11111111-1111-4111-8111-111111111111', '/money'])('restores protected notification path %s', path => {
 handleAuthRedirect({ returnTo: path }); expect(window.location.pathname).toBe(path);
});
it.each(['//outside.example', '/sessions/invalid', '/money?next=https://evil', '/admin'])('rejects unsafe or unexpected OAuth return %s', path => {
 handleAuthRedirect({ returnTo: path }); expect(window.location.pathname).toBe('/');
});
it('preserves the original settlement query through sign-in', () => {
 const path = '/sessions/11111111-1111-4111-8111-111111111111/settlement?settlement_id=22222222-2222-4222-8222-222222222222';
 handleAuthRedirect({ returnTo: path });
 expect(window.location.pathname + window.location.search).toBe(path);
});

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

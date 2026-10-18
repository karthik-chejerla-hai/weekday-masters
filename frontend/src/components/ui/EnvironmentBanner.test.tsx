import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EnvironmentBanner } from './EnvironmentBanner';

afterEach(() => { cleanup(); vi.unstubAllEnvs(); });
describe('EnvironmentBanner', () => {
  it('identifies dev and its source commit', () => {
    vi.stubEnv('VITE_APP_ENVIRONMENT', 'dev');
    vi.stubEnv('VITE_RELEASE_SHA', 'abcdef0123456789');
    render(<EnvironmentBanner />);
    expect(screen.getByRole('status')).toHaveTextContent('DEV · Test data · abcdef0');
  });
  it('does not label production as dev', () => {
    vi.stubEnv('VITE_APP_ENVIRONMENT', 'prod');
    render(<EnvironmentBanner />);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});

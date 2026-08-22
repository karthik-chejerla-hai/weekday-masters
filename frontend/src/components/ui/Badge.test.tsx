import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Badge from './Badge';

describe('Badge Component', () => {
  it('renders children correctly', () => {
    render(<Badge>Active</Badge>);
    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('applies default variant classes', () => {
    render(<Badge>Default</Badge>);
    const badge = screen.getByText('Default');
    expect(badge).toHaveClass('bg-slate-100', 'text-slate-700');
  });

  it('applies success variant classes', () => {
    render(<Badge variant="success">Confirmed</Badge>);
    const badge = screen.getByText('Confirmed');
    expect(badge).toHaveClass('bg-green-100', 'text-green-700');
  });

  it('applies warning variant classes', () => {
    render(<Badge variant="warning">Waitlisted</Badge>);
    const badge = screen.getByText('Waitlisted');
    expect(badge).toHaveClass('bg-amber-100', 'text-amber-700');
  });

  it('applies danger variant classes', () => {
    render(<Badge variant="danger">Cancelled</Badge>);
    const badge = screen.getByText('Cancelled');
    expect(badge).toHaveClass('bg-red-100', 'text-red-700');
  });

  it('applies info variant classes', () => {
    render(<Badge variant="info">Notice</Badge>);
    const badge = screen.getByText('Notice');
    expect(badge).toHaveClass('bg-blue-100', 'text-blue-700');
  });
});

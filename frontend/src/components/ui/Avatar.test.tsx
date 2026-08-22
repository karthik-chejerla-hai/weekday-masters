import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Avatar from './Avatar';

describe('Avatar Component', () => {
  it('renders initials when no src is provided', () => {
    render(<Avatar name="John Doe" />);
    expect(screen.getByText('JD')).toBeInTheDocument();
  });

  it('renders single initial for single word name', () => {
    render(<Avatar name="Alice" />);
    expect(screen.getByText('A')).toBeInTheDocument();
  });

  it('renders image when src is provided', () => {
    render(<Avatar name="John Doe" src="https://example.com/avatar.jpg" />);
    const img = screen.getByRole('img', { name: 'John Doe' });
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute('src', 'https://example.com/avatar.jpg');
  });

  it('optimizes googleusercontent URLs with size parameter', () => {
    render(<Avatar name="John Doe" src="https://lh3.googleusercontent.com/a/abc=s96-c" size="sm" />);
    const img = screen.getByRole('img', { name: 'John Doe' });
    expect(img).toHaveAttribute('src', 'https://lh3.googleusercontent.com/a/abc=s64');
  });

  it('applies correct size classes', () => {
    const { container } = render(<Avatar name="Jane Doe" size="lg" />);
    const avatar = container.firstChild as HTMLElement;
    expect(avatar).toHaveClass('w-16', 'h-16', 'text-xl');
  });
});

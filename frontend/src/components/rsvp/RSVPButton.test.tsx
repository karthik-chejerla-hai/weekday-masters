import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import RSVPButton from './RSVPButton';

describe('RSVPButton Component', () => {
  it('renders three RSVP buttons with standard labels', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} />);

    expect(screen.getByText("I'm In")).toBeInTheDocument();
    expect(screen.getByText('Maybe')).toBeInTheDocument();
    expect(screen.getByText("Can't Make It")).toBeInTheDocument();
  });

  it('shows "Join Waitlist" when session is full', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} isFull={true} />);

    expect(screen.getByText('Join Waitlist')).toBeInTheDocument();
  });

  it('shows "Waitlisted #2" when user is waitlisted at position 2', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} currentStatus="waitlisted" waitlistPosition={2} />);

    expect(screen.getByText('Waitlisted #2')).toBeInTheDocument();
  });

  it('triggers onRSVP callback with correct status when clicked', async () => {
    const handleRSVP = vi.fn().mockResolvedValue(undefined);
    render(<RSVPButton onRSVP={handleRSVP} />);

    const inBtn = screen.getByText("I'm In").closest('button');
    expect(inBtn).toBeInTheDocument();
    fireEvent.click(inBtn!);

    expect(handleRSVP).toHaveBeenCalledWith('in');
  });

  it('disables all buttons when disabled prop is true', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} disabled={true} />);

    const buttons = screen.getAllByRole('button');
    buttons.forEach((btn) => {
      expect(btn).toBeDisabled();
    });
  });

  it('applies active styling for currentStatus "in"', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} currentStatus="in" />);

    const inBtn = screen.getByText("I'm In").closest('button');
    expect(inBtn).toHaveClass('bg-green-600', 'text-white');
  });

  it('applies active styling for currentStatus "maybe"', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} currentStatus="maybe" />);

    const maybeBtn = screen.getByText('Maybe').closest('button');
    expect(maybeBtn).toHaveClass('bg-amber-500', 'text-white');
  });

  it('applies active styling for currentStatus "out"', () => {
    const handleRSVP = vi.fn();
    render(<RSVPButton onRSVP={handleRSVP} currentStatus="out" />);

    const outBtn = screen.getByText("Can't Make It").closest('button');
    expect(outBtn).toHaveClass('bg-red-600', 'text-white');
  });
});

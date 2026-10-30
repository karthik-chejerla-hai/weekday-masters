import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AdminRSVPForm from './AdminRSVPForm';
import { api } from '../../services/api';
import type { GuestRSVP, User } from '../../types';
vi.mock('../../services/api', () => ({ api: { listMembers: vi.fn(), saveGuestRSVP: vi.fn(), adminAddRSVP: vi.fn() } }));
const payer = { id: 'payer', name: 'Pat Smith', membership_status: 'approved' } as User;
const guest = { id: 'g1', guest_name: 'Alex', charged_to_user_id: 'payer', charged_to_user: payer, status: 'in' } as GuestRSVP;
const onSaved = vi.fn().mockResolvedValue(undefined);
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listMembers).mockResolvedValue([payer]);
  vi.mocked(api.saveGuestRSVP).mockResolvedValue(guest);
});
async function form(props: Partial<React.ComponentProps<typeof AdminRSVPForm>> = {}) {
  render(<AdminRSVPForm sessionId="s1" rsvps={[]} deadlinePassed={false} isFull={false} onSaved={onSaved} {...props} />);
  await screen.findByRole('option', { name: 'Pat' });
  return userEvent.setup();
}
it('adds an IN guest with a payer, then edits that record without changing the payer RSVP', async () => {
  const user = await form({ deadlinePassed: true });
  await user.selectOptions(screen.getByLabelText('Player type'), 'guest');
  expect(screen.getByLabelText('Response')).toHaveValue('in');
  expect(screen.queryByRole('option', { name: 'Maybe' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Save guest RSVP' })).toBeDisabled();
  await user.type(screen.getByLabelText('Guest name'), ' Alex ');
  await user.selectOptions(screen.getByLabelText('Charged to member'), 'payer');
  await user.click(screen.getByRole('button', { name: 'Save guest RSVP' }));
  expect(api.saveGuestRSVP).toHaveBeenCalledWith('s1', { guest_name: 'Alex', charged_to_user_id: 'payer', status: 'in' }, undefined);
  expect(api.adminAddRSVP).not.toHaveBeenCalled();
  expect(await screen.findByRole('status')).toHaveTextContent("Alex's RSVP saved as In.");
  await user.selectOptions(screen.getByLabelText('Response'), 'out');
  await user.click(screen.getByRole('button', { name: 'Save guest RSVP' }));
  expect(api.saveGuestRSVP).toHaveBeenLastCalledWith('s1', { guest_name: 'Alex', charged_to_user_id: 'payer', status: 'out' }, 'g1');
});
it('allows editing a confirmed guest at capacity and setting them OUT', async () => {
  const user = await form({ guests: [guest], isFull: true });
  await user.selectOptions(screen.getByLabelText('Player type'), 'guest');
  await user.selectOptions(screen.getByLabelText('Guest RSVP'), 'g1');
  expect(screen.getByLabelText('Guest name')).toHaveValue('Alex');
  expect(screen.getByLabelText('Charged to member')).toHaveValue('payer');
  expect(screen.getByRole('button', { name: 'Save guest RSVP' })).toBeEnabled();
  await user.selectOptions(screen.getByLabelText('Response'), 'out');
  await user.click(screen.getByRole('button', { name: 'Save guest RSVP' }));
  expect(api.saveGuestRSVP).toHaveBeenCalledWith('s1', { guest_name: 'Alex', charged_to_user_id: 'payer', status: 'out' }, 'g1');
});
it('blocks a new IN guest when full but permits OUT', async () => {
  const user = await form({ isFull: true });
  await user.selectOptions(screen.getByLabelText('Player type'), 'guest');
  await user.type(screen.getByLabelText('Guest name'), 'Alex');
  await user.selectOptions(screen.getByLabelText('Charged to member'), 'payer');
  expect(screen.getByRole('button', { name: 'Save guest RSVP' })).toBeDisabled();
  await user.selectOptions(screen.getByLabelText('Response'), 'out');
  expect(screen.getByRole('button', { name: 'Save guest RSVP' })).toBeEnabled();
});
it('reports a failed save', async () => {
  vi.mocked(api.saveGuestRSVP).mockRejectedValue(new Error('offline'));
  const user = await form({ guests: [guest] });
  await user.selectOptions(screen.getByLabelText('Player type'), 'guest');
  await user.selectOptions(screen.getByLabelText('Guest RSVP'), 'g1');
  await user.click(screen.getByRole('button', { name: 'Save guest RSVP' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Could not save this RSVP');
  expect(onSaved).not.toHaveBeenCalled();
});
it('does not offer guests after the session starts', async () => {
  await form({ allowGuests: false });
  expect(screen.queryByLabelText('Player type')).not.toBeInTheDocument();
});

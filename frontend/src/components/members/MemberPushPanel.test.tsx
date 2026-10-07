import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api, type MemberPushStatus } from '../../services/api';
import MemberPushPanel from './MemberPushPanel';
import type { User } from '../../types';

vi.mock('../../services/api', () => ({ api: { getMemberPushStatus: vi.fn() } }));
const member = { id: 'member-1', name: 'Alex Test', nickname: 'Alex', membership_status: 'approved' } as User;
const now = '2026-10-07T01:00:00Z';
const saved: MemberPushStatus = {
  preferences: { id: 'p1', user_id: member.id, created_at: now, updated_at: now,
    push_enabled: false, push_session_reminders: true, push_rsvp_deadlines: true,
    push_waitlist_updates: false, push_admin_announcements: true, push_balance_alerts: false },
  devices: [{ id: 'd1', device_name: 'Phone', created_at: now, last_registered_at: now },
    { id: 'd2', device_name: '', created_at: now, last_registered_at: now }],
};
beforeEach(() => vi.resetAllMocks());

it('loads only on request and separates account settings from device registration', async () => {
  vi.mocked(api.getMemberPushStatus).mockResolvedValue(saved);
  const user = userEvent.setup();
  render(<MemberPushPanel member={member} />);
  expect(api.getMemberPushStatus).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Push alerts and devices' }));
  expect(await screen.findByText('Account push: Off')).toBeInTheDocument();
  expect(api.getMemberPushStatus).toHaveBeenCalledWith(member.id);
  expect(screen.getByText('2 registered devices')).toBeInTheDocument();
  expect(screen.getByText('Phone')).toBeInTheDocument();
  expect(screen.getByText('Unnamed browser 2')).toBeInTheDocument();
  expect(screen.getByText('Balance alerts').nextElementSibling).toHaveTextContent('Off');
  expect(screen.getByText('Session reminders').nextElementSibling).toHaveTextContent('On');
  expect(screen.getByText(/Account push is off/)).toBeInTheDocument();
  expect(screen.getByText(/does not confirm current phone or browser settings/)).toBeInTheDocument();
  vi.mocked(api.getMemberPushStatus).mockResolvedValue({ preferences: null, devices: [] });
  await user.click(screen.getByRole('button', { name: 'Refresh' }));
  expect(await screen.findByText('No saved push settings')).toBeInTheDocument();
  expect(screen.getByText(/No registered devices/)).toBeInTheDocument();
  expect(screen.queryByText('Phone')).not.toBeInTheDocument();
});

it('shows a load error instead of claiming no devices and allows retry', async () => {
  vi.mocked(api.getMemberPushStatus).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ preferences: null, devices: [] });
  const user = userEvent.setup();
  render(<MemberPushPanel member={member} />);
  await user.click(screen.getByRole('button', { name: 'Push alerts and devices' }));
  const alert = await screen.findByRole('alert');
  expect(screen.queryByText(/No registered devices/)).not.toBeInTheDocument();
  await user.click(within(alert).getByRole('button', { name: 'Try again' }));
  expect(await screen.findByText('No saved push settings')).toBeInTheDocument();
});

import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import InvitationPanel from './InvitationPanel';
import { api } from '../../services/api';
import type { InvitationPreview, User } from '../../types';

vi.mock('../../services/api', () => ({ api: { previewInvitation: vi.fn(), sendInvitation: vi.fn() } }));
const member = { id: 'member-1', name: 'Hari Prasad', email: 'hari@example.com', nickname: '' } as User;
const preview: InvitationPreview = {
  subject: 'Welcome to Rally', html: '<p>Hi Hari</p>', text: 'Hi Hari', login_url: 'https://rally.example/welcome',
  recipient_email: member.email, from_email: 'club@example.com', test_recipient: 'admin@example.com',
  can_send: true, can_test: true,
};
const accepted = { id: 'attempt-1', user_id: member.id, recipient_email: member.email, is_test: false, status: 'accepted' as const, message: 'Accepted by provider. Inbox delivery is not confirmed.', created_at: '2026-10-04T06:00:00Z' };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.previewInvitation).mockResolvedValue({ ...preview }); });

it('renders the exact sandboxed email, recipient and link without sending', async () => {
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  const frame = await screen.findByTitle('Invitation email for Hari');
  expect(frame).toHaveAttribute('srcdoc', preview.html);
  expect(frame).toHaveAttribute('sandbox', '');
  expect(screen.getByRole('link', { name: /open sign-in link/i })).toHaveAttribute('href', preview.login_url);
  expect(api.sendInvitation).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Mobile' }));
  expect(frame).toHaveClass('max-w-[360px]');
});

it('allows an admin test while member invitations remain paused', async () => {
  vi.mocked(api.previewInvitation).mockResolvedValue({ ...preview, can_send: false, send_blocked_reason: 'Member invitations are paused.' });
  vi.mocked(api.sendInvitation).mockResolvedValue({ ...accepted, is_test: true, recipient_email: preview.test_recipient });
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  expect(await screen.findByRole('button', { name: 'Send invitation' })).toBeDisabled();
  await userEvent.click(screen.getByRole('button', { name: 'Send test to me' }));
  await waitFor(() => expect(api.sendInvitation).toHaveBeenCalledWith(member.id, expect.any(String), member.email, true));
  expect(await screen.findByRole('status')).toHaveTextContent('Test email: Accepted');
  expect(screen.queryByRole('button', { name: 'Resend invitation' })).not.toBeInTheDocument();
});

it('sends only after explicit action, then offers a resend', async () => {
  vi.mocked(api.sendInvitation).mockResolvedValue(accepted);
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Send invitation' }));
  await waitFor(() => expect(api.sendInvitation).toHaveBeenCalledWith(member.id, expect.any(String), member.email, false));
  expect(await screen.findByRole('button', { name: 'Resend invitation' })).toBeEnabled();
  expect(screen.getByRole('status')).toHaveTextContent('Inbox delivery is not confirmed');
});

it('uses the same request ID to check a lost response, without another send attempt', async () => {
  vi.mocked(api.sendInvitation).mockRejectedValueOnce(new Error('connection lost')).mockResolvedValueOnce(accepted);
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Send invitation' }));
  await screen.findByRole('alert');
  expect(screen.getByRole('button', { name: 'Send invitation' })).toBeDisabled();
  await userEvent.click(screen.getByRole('button', { name: 'Check result' }));
  await screen.findByRole('button', { name: 'Resend invitation' });
  expect(vi.mocked(api.sendInvitation).mock.calls[1]).toEqual(vi.mocked(api.sendInvitation).mock.calls[0]);
});

it('shows provider rejection and keeps the status truthful', async () => {
  vi.mocked(api.sendInvitation).mockResolvedValue({ ...accepted, status: 'failed', message: 'Provider rejected the email.' });
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Send invitation' }));
  expect(await screen.findByRole('status')).toHaveTextContent('Provider rejected');
  expect(screen.getByText(/Send failed/)).toBeInTheDocument();
  expect(screen.queryByText(/Accepted for delivery/)).not.toBeInTheDocument();
});

it('reports preview failure and blocks missing-provider sends', async () => {
  vi.mocked(api.previewInvitation).mockRejectedValue({ response: { data: { error: 'Member already signed in.' } } });
  render(<InvitationPanel member={member} onClose={vi.fn()} />);
  expect(await screen.findByRole('alert')).toHaveTextContent('Member already signed in.');
  expect(screen.queryByRole('button', { name: 'Send invitation' })).not.toBeInTheDocument();
});

import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import NotificationSettings from './NotificationSettings';
import { notificationService } from '../../services/notifications';
vi.mock('../../context/useAuth', () => ({ useAuth: () => ({ user: { id: 'member' } }) }));
vi.mock('../../services/notifications', () => ({ notificationService: {
 isPushSupported: vi.fn(() => true), getPermissionStatus: vi.fn(() => 'granted'),
 restorePushNotifications: vi.fn(async () => true), enablePushNotifications: vi.fn(async () => true), disablePushNotifications: vi.fn(async () => {}), sendTestPush: vi.fn(async () => ({ accepted_devices: 1, attempted_devices: 1 }))
} }));
beforeEach(() => { vi.clearAllMocks(); vi.mocked(notificationService.restorePushNotifications).mockResolvedValue(true); vi.mocked(notificationService.isPushSupported).mockReturnValue(true); vi.mocked(notificationService.getPermissionStatus).mockReturnValue('granted'); vi.mocked(notificationService.disablePushNotifications).mockResolvedValue(); });
it('shows one device control and no account or category switches', async () => {
 render(<NotificationSettings />); expect(await screen.findByRole('button', { name: 'Disable on this device' })).toBeInTheDocument();
 expect(screen.queryByRole('switch')).not.toBeInTheDocument(); expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
});
it('requires explicit enable after startup finds no registration', async () => {
 vi.mocked(notificationService.restorePushNotifications).mockResolvedValue(false);
 render(<NotificationSettings />); fireEvent.click(await screen.findByRole('button', { name: 'Enable on this device' }));
 await waitFor(() => expect(notificationService.enablePushNotifications).toHaveBeenCalledWith('member'));
 expect(screen.getByRole('status', { name: 'Device push status' })).toHaveTextContent('Enabled');
});
it('does not claim disable success when backend or Firebase cleanup fails', async () => {
 vi.mocked(notificationService.disablePushNotifications).mockRejectedValue(new Error('offline'));
 render(<NotificationSettings />); fireEvent.click(await screen.findByRole('button', { name: 'Disable on this device' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('has not confirmed success'); expect(screen.queryByText('Push disabled on this device.')).not.toBeInTheDocument();
});
it.each(['denied', 'unsupported'] as const)('permits explicit disable despite %s browser permission', async permission => {
 vi.mocked(notificationService.getPermissionStatus).mockReturnValue(permission);
 render(<NotificationSettings />); const button = await screen.findByRole('button', { name: 'Disable on this device' });expect(button).toBeEnabled();fireEvent.click(button);
 await waitFor(() => expect(notificationService.disablePushNotifications).toHaveBeenCalledWith('member'));
});
it('shows unsupported help without requesting permission', async () => {
 vi.mocked(notificationService.isPushSupported).mockReturnValue(false);vi.mocked(notificationService.restorePushNotifications).mockResolvedValue(false);
 render(<NotificationSettings />); expect(await screen.findByRole('button', { name: 'Enable on this device' })).toBeDisabled();expect(screen.getByText(/Push is unavailable/)).toBeInTheDocument();
});
it('shows load failures explicitly and lets members check again', async () => {
 vi.mocked(notificationService.restorePushNotifications).mockRejectedValueOnce(new Error('offline'));
 render(<NotificationSettings />); expect(await screen.findByRole('alert')).toHaveTextContent('Could not check');fireEvent.click(screen.getByRole('button', { name: 'Check again' }));
 expect(await screen.findByRole('button', { name: 'Disable on this device' })).toBeInTheDocument();
});
it('retains admin self-test only', async () => {
 render(<NotificationSettings isAdmin />);fireEvent.click(await screen.findByRole('button', { name: 'Send test notification' }));
 expect(await screen.findByText('Test notification accepted by 1 of 1 registered devices.')).toBeInTheDocument();
});

it('describes reminders using each session deadline rather than a fixed close time', async () => {
 render(<NotificationSettings />);
 await screen.findByRole('button', { name: 'Disable on this device' });
 const policy = screen.getByText(/Check each session's RSVP deadline/);
 expect(policy).toHaveTextContent('96 and 60 hours');
 expect(policy).toHaveTextContent('its RSVP window is open');
 expect(policy).not.toHaveTextContent('48 hours');
});

it('shows the device setup status only once after enabling and checking again', async () => {
 vi.mocked(notificationService.restorePushNotifications).mockResolvedValueOnce(false).mockResolvedValue(true);
 render(<NotificationSettings />);
 fireEvent.click(await screen.findByRole('button', { name: 'Enable on this device' }));
 await screen.findByRole('button', { name: 'Disable on this device' });
 expect(screen.getAllByText('Enabled', { exact: true })).toHaveLength(1);
 fireEvent.click(screen.getByRole('button', { name: 'Check again' }));
 expect(await screen.findByText('Device check complete.')).toBeInTheDocument();
 expect(screen.getAllByText('Enabled', { exact: true })).toHaveLength(1);
});

it('makes a manual recheck visible and prevents another action until it finishes', async () => {
 let resolve!: (value: boolean) => void;
 const pending = new Promise<boolean>(done => { resolve = done; });
 vi.mocked(notificationService.restorePushNotifications).mockResolvedValueOnce(true).mockReturnValueOnce(pending);
 render(<NotificationSettings isAdmin />);
 fireEvent.click(await screen.findByRole('button', { name: 'Check again' }));
 expect(screen.getByRole('button', { name: 'Checking…' })).toBeDisabled();
 expect(screen.getByRole('status', { name: 'Device push status' })).toHaveTextContent('Checking…');
 expect(screen.getByRole('button', { name: 'Disable on this device' })).toBeDisabled();
 expect(screen.getByRole('button', { name: 'Send test notification' })).toBeDisabled();
 resolve(true);
 expect(await screen.findByText('Device check complete.')).toBeInTheDocument();
 expect(screen.getByRole('button', { name: 'Check again' })).toBeEnabled();
});

it('shows a green enabled badge beside the heading for a confirmed device', async () => {
 render(<NotificationSettings />);
 await screen.findByRole('button', { name: 'Disable on this device' });
 const badge = screen.getByRole('status', { name: 'Device push status' });
 expect(badge).toHaveTextContent(/^Enabled$/);
 expect(badge).toHaveClass('text-emerald-700');
 expect(badge.parentElement).toContainElement(screen.getByRole('heading', { name: 'Push notifications' }));
 expect(screen.queryByText(/^Push set up on this device\.?$/)).not.toBeInTheDocument();
});

it('shows disabled when this browser has no registration', async () => {
 vi.mocked(notificationService.restorePushNotifications).mockResolvedValue(false);
 render(<NotificationSettings />);
 await screen.findByRole('button', { name: 'Enable on this device' });
 expect(screen.getByRole('status', { name: 'Device push status' })).toHaveTextContent(/^Disabled$/);
});

it('does not retain a green status after a failed device recheck', async () => {
 vi.mocked(notificationService.restorePushNotifications).mockResolvedValueOnce(true).mockRejectedValueOnce(new Error('offline'));
 render(<NotificationSettings />);
 fireEvent.click(await screen.findByRole('button', { name: 'Check again' }));
 await screen.findByRole('alert');
 const badge = screen.getByRole('status', { name: 'Device push status' });
 expect(badge).toHaveTextContent(/^Not confirmed$/);
 expect(badge).not.toHaveClass('text-emerald-700');
});

it('shows blocked instead of enabled when browser permission is denied', async () => {
 vi.mocked(notificationService.getPermissionStatus).mockReturnValue('denied');
 render(<NotificationSettings />);
 await screen.findByRole('button', { name: 'Disable on this device' });
 expect(screen.getByRole('status', { name: 'Device push status' })).toHaveTextContent(/^Blocked$/);
});

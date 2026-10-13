import { act, render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import PushNotifications from './PushNotifications';
import { notificationService } from '../../services/notifications';
vi.mock('../../context/useAuth', () => ({ useAuth: () => ({ user: { id: 'member' }, isApproved: true }) }));
vi.mock('../../services/notifications', () => ({ notificationService: {
 isPushSupported: vi.fn(() => true), getPermissionStatus: vi.fn(() => 'granted'), isDeviceDisabled: vi.fn(() => false), restorePushNotifications: vi.fn(async () => true),
 enablePushNotifications: vi.fn(async () => true), setupForegroundHandler: vi.fn(() => vi.fn())
} }));
const stored = new Map<string,string>();
Object.defineProperty(window,'localStorage',{configurable:true,value:{getItem:(key:string)=>stored.get(key)||null,setItem:(key:string,value:string)=>stored.set(key,value),removeItem:(key:string)=>stored.delete(key)}});
beforeEach(() => { vi.clearAllMocks();stored.clear();vi.mocked(notificationService.getPermissionStatus).mockReturnValue('granted');vi.mocked(notificationService.isDeviceDisabled).mockReturnValue(false);vi.mocked(notificationService.restorePushNotifications).mockResolvedValue(true); });
function show(path='/dashboard') { return render(<MemoryRouter initialEntries={[path]}><PushNotifications /></MemoryRouter>); }
it('restores existing enrollment without explicitly enrolling and shows actionable messages', async () => {
 show();await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
 expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
 act(() => vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0]('Charge','Split ready',{type:'session_charge',session_id:'11111111-1111-4111-8111-111111111111'}));
 expect(screen.getByRole('link')).toHaveAttribute('href','/sessions/11111111-1111-4111-8111-111111111111/settlement');
});
it('permission prompt happens only from explicit enable', async () => {
 vi.mocked(notificationService.getPermissionStatus).mockReturnValue('default');show();fireEvent.click(await screen.findByRole('button',{name:'Enable on this device'}));
 await waitFor(() => expect(notificationService.enablePushNotifications).toHaveBeenCalledWith('member'));
});
it('respects explicit disable across refresh and other tabs', async () => {
 const unsubscribe=vi.fn();vi.mocked(notificationService.setupForegroundHandler).mockReturnValue(unsubscribe);show();await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
 vi.mocked(notificationService.isDeviceDisabled).mockReturnValue(true);act(() => window.dispatchEvent(new Event('storage')));
 await waitFor(() => expect(unsubscribe).toHaveBeenCalled());expect(screen.queryByRole('button',{name:'Enable on this device'})).not.toBeInTheDocument();
});
it('does not repeatedly prompt after explicit disable even if permission resets', async () => {
 vi.mocked(notificationService.isDeviceDisabled).mockReturnValue(true);vi.mocked(notificationService.getPermissionStatus).mockReturnValue('default');show();
 await act(async () => {});expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();expect(screen.queryByRole('button',{name:'Enable on this device'})).not.toBeInTheDocument();
});
it.each(['balance_low','balance_negative'])('routes foreground %s to money', async type => {
 show();await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());act(() => vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0]('Balance','Top up',{type}));expect(screen.getByRole('link')).toHaveAttribute('href','/money');
});

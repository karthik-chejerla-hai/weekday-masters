import { beforeEach, expect, it, vi } from 'vitest';
import { api, type PushEnrollment } from './api';
import { getExistingPushToken, requestNotificationPermission, deletePushToken } from './firebase';
import { restorePushDevice, enablePushDevice, disablePushDevice, preparePushAccount } from './push-device';
vi.mock('./api', () => ({ api: { getPushDevices: vi.fn(), adoptPushDevice: vi.fn(), enrollPushDevice: vi.fn(), refreshPushDevice: vi.fn(), revokePushDevice: vi.fn(), revokePushDeviceProof: vi.fn() } }));
vi.mock('./firebase', () => ({ getExistingPushToken: vi.fn(async () => 'token'), requestNotificationPermission: vi.fn(async () => 'token'), deletePushToken: vi.fn(async () => {}) }));
const values = new Map<string,string>();
Object.defineProperty(window,'localStorage',{configurable:true,value:{getItem:(key:string)=>values.get(key)||null,setItem:(key:string,value:string)=>values.set(key,value)}});
const device: PushEnrollment = { id:'device', installation_id:'installation', generation:1, device_name:'Phone', state:'active' };
function seed(state:PushEnrollment['state']='active') { values.set('rally:push-installation:v2',JSON.stringify({owner:'member',installation:'installation',secret:'ab'.repeat(32),device:{...device,state},disabled:state==='revoked'})); }
beforeEach(() => { vi.resetAllMocks();values.clear();vi.mocked(api.getPushDevices).mockResolvedValue([device]);vi.mocked(api.refreshPushDevice).mockResolvedValue(device);vi.mocked(getExistingPushToken).mockResolvedValue('token');vi.mocked(requestNotificationPermission).mockResolvedValue('token');vi.mocked(deletePushToken).mockResolvedValue();vi.mocked(api.revokePushDevice).mockResolvedValue();vi.mocked(api.revokePushDeviceProof).mockResolvedValue(); });
it('silently adopts a same-owner legacy token without enrolling or requesting permission', async () => {
 vi.mocked(api.getPushDevices).mockResolvedValue([]);vi.mocked(api.adoptPushDevice).mockImplementation(async input=>({...device,installation_id:input.installation_id}));
 expect(await restorePushDevice('member')).toBe(true);expect(api.adoptPushDevice).toHaveBeenCalled();expect(api.enrollPushDevice).not.toHaveBeenCalled();expect(requestNotificationPermission).not.toHaveBeenCalled();
});
it('never automatically enrolls a missing legacy token', async () => {
 vi.mocked(api.getPushDevices).mockResolvedValue([]);vi.mocked(api.adoptPushDevice).mockRejectedValue({response:{status:404}});
 expect(await restorePushDevice('member')).toBe(false);expect(api.enrollPushDevice).not.toHaveBeenCalled();
});
it('refreshes known active enrollment with generation and scoped ownership', async () => {
 seed();expect(await restorePushDevice('member')).toBe(true);expect(api.refreshPushDevice).toHaveBeenCalledWith('device',{installation_id:'installation',generation:1,token:'token'});
});
it('keeps explicit disable across reload and does not refresh', async () => {
 seed();await disablePushDevice('member');expect(api.revokePushDevice).toHaveBeenCalledWith('device',1);expect(await restorePushDevice('member')).toBe(false);expect(api.refreshPushDevice).not.toHaveBeenCalled();
});
it('explicit enable alone revives a tombstone with current server generation', async () => {
 seed('revoked');vi.mocked(api.getPushDevices).mockResolvedValue([{...device,state:'revoked',generation:2}]);vi.mocked(api.enrollPushDevice).mockResolvedValue({...device,generation:3});
 expect(await enablePushDevice('member')).toBe(true);expect(api.enrollPushDevice).toHaveBeenCalledWith(expect.objectContaining({generation:2,revocation_secret:'ab'.repeat(32)}));
});
it('switches accounts only after proof revocation and Firebase cleanup', async () => {
 seed();await preparePushAccount('other');expect(api.revokePushDeviceProof).toHaveBeenCalledWith('installation','ab'.repeat(32),1);expect(deletePushToken).toHaveBeenCalledOnce();
 expect(JSON.parse(values.get('rally:push-installation:v2')!).owner).toBe('other');expect(api.enrollPushDevice).not.toHaveBeenCalled();
});
it('blocks account switch after failed cleanup and retries without repeating server revocation', async () => {
 seed();vi.mocked(deletePushToken).mockRejectedValueOnce(new Error('offline'));
 await expect(preparePushAccount('other')).rejects.toThrow('offline');expect(JSON.parse(values.get('rally:push-installation:v2')!).owner).toBe('member');
 await preparePushAccount('other');expect(api.revokePushDeviceProof).toHaveBeenCalledOnce();expect(JSON.parse(values.get('rally:push-installation:v2')!).owner).toBe('other');
});
it('ordinary same-account sign-in preserves registration', async () => { seed();await preparePushAccount('member');expect(api.revokePushDeviceProof).not.toHaveBeenCalled();expect(deletePushToken).not.toHaveBeenCalled(); });
it('a failed scoped revoke cannot report local disable success', async () => {
 seed();vi.mocked(api.revokePushDevice).mockRejectedValue(new Error('offline'));await expect(disablePushDevice('member')).rejects.toThrow('offline');
 expect(JSON.parse(values.get('rally:push-installation:v2')!).disabled).toBe(false);expect(deletePushToken).not.toHaveBeenCalled();
});
it('repairs invalidated enrollment without a permission prompt', async () => {seed();vi.mocked(api.getPushDevices).mockResolvedValue([{...device,state:'invalidated'}]);expect(await restorePushDevice('member')).toBe(true);expect(deletePushToken).toHaveBeenCalledOnce();expect(requestNotificationPermission).not.toHaveBeenCalled();});

it('an old tab cannot silently reclaim the new account installation', async () => {
 seed();await preparePushAccount('other');vi.mocked(api.revokePushDeviceProof).mockClear();
 await expect(restorePushDevice('member')).rejects.toThrow('Another account');expect(api.revokePushDeviceProof).not.toHaveBeenCalled();
});

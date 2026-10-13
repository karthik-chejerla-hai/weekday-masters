import { beforeEach, expect, it, vi } from 'vitest';
import { api, type PushEnrollment } from './api';
import { getExistingPushToken, requestNotificationPermission, deletePushToken } from './firebase';
import { restorePushDevice, enablePushDevice, disablePushDevice, preparePushAccount } from './push-device';
vi.mock('./api', () => ({ api: { getPushDevices: vi.fn(), adoptPushDevice: vi.fn(), enrollPushDevice: vi.fn(), refreshPushDevice: vi.fn(), revokePushDevice: vi.fn(), revokePushDeviceProof: vi.fn(), revokeForeignLegacyToken: vi.fn() } }));
vi.mock('./firebase', () => ({ getExistingPushToken: vi.fn(async () => 'token'), requestNotificationPermission: vi.fn(async () => 'token'), deletePushToken: vi.fn(async () => {}) }));
const values = new Map<string,string>();
Object.defineProperty(window,'localStorage',{configurable:true,value:{getItem:(key:string)=>values.get(key)||null,setItem:(key:string,value:string)=>values.set(key,value)}});
const device: PushEnrollment = { id:'device', installation_id:'installation', generation:1, device_name:'Phone', state:'active' };
function seed(state:PushEnrollment['state']='active') { values.set('rally:push-installation:v2',JSON.stringify({owner:'member',installation:'installation',secret:'ab'.repeat(32),device:{...device,state},disabled:state==='revoked'})); }
beforeEach(() => { vi.resetAllMocks();values.clear();vi.mocked(api.getPushDevices).mockResolvedValue([device]);vi.mocked(api.refreshPushDevice).mockResolvedValue(device);vi.mocked(getExistingPushToken).mockResolvedValue('token');vi.mocked(requestNotificationPermission).mockResolvedValue('token');vi.mocked(deletePushToken).mockResolvedValue();vi.mocked(api.revokePushDevice).mockResolvedValue();vi.mocked(api.revokePushDeviceProof).mockResolvedValue();vi.mocked(api.revokeForeignLegacyToken).mockResolvedValue({cleanup_required:false}); });
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

const conflict = (generation: number) => ({ response: { status: 409, data: { code: 'device_generation_changed', details: { current_generation: generation } } } });
it.each(['enroll', 'adopt'])('recovers lost %s response before switching accounts', async mode => {
 vi.mocked(api.getPushDevices).mockResolvedValue([]);
 if (mode === 'enroll') {
  vi.mocked(api.enrollPushDevice).mockRejectedValue(new Error('accepted but response lost'));
  await expect(enablePushDevice('member')).rejects.toThrow('response lost');
 } else {
  vi.mocked(api.adoptPushDevice).mockRejectedValue(new Error('accepted but response lost'));
  await expect(restorePushDevice('member')).rejects.toThrow('response lost');
 }
 const pending=JSON.parse(values.get('rally:push-installation:v2')!);
 expect(pending.registrationAttempted).toBe(true);expect(pending.device).toBeUndefined();
 vi.mocked(api.revokePushDeviceProof).mockRejectedValueOnce(conflict(1)).mockResolvedValueOnce();
 await preparePushAccount('other');
 expect(api.revokePushDeviceProof).toHaveBeenNthCalledWith(1,pending.installation,pending.secret,0);
 expect(api.revokePushDeviceProof).toHaveBeenNthCalledWith(2,pending.installation,pending.secret,1);
 expect(deletePushToken).toHaveBeenCalledOnce();expect(JSON.parse(values.get('rally:push-installation:v2')!).owner).toBe('other');
});
it('recovers a lost refresh generation only inside persisted explicit cleanup intent', async () => {
 seed();vi.mocked(api.refreshPushDevice).mockRejectedValue(new Error('response lost'));
 await expect(restorePushDevice('member')).rejects.toThrow();
 vi.mocked(api.revokePushDeviceProof).mockImplementationOnce(async () => {
  expect(JSON.parse(values.get('rally:push-installation:v2')!).cleanupIntent).toBeTruthy();throw conflict(2);
 }).mockResolvedValueOnce();
 await preparePushAccount('other');expect(api.revokePushDeviceProof).toHaveBeenNthCalledWith(2,'installation','ab'.repeat(32),2);
});
it('bounds repeated CAS conflicts and retains cleanup ownership for explicit retry', async () => {
 seed();let generation=1;vi.mocked(api.revokePushDeviceProof).mockImplementation(async () => {throw conflict(++generation);});
 await expect(preparePushAccount('other')).rejects.toThrow('kept changing');expect(api.revokePushDeviceProof).toHaveBeenCalledTimes(3);
 const pending=JSON.parse(values.get('rally:push-installation:v2')!);expect(pending.owner).toBe('member');expect(pending.cleanupIntent).toBeTruthy();expect(deletePushToken).not.toHaveBeenCalled();
 vi.mocked(api.revokePushDeviceProof).mockResolvedValue();await preparePushAccount('other');expect(JSON.parse(values.get('rally:push-installation:v2')!).owner).toBe('other');
});
it('wrong proof errors cannot supply or retry a generation', async () => {
 seed();vi.mocked(api.revokePushDeviceProof).mockRejectedValue({response:{status:404,data:{code:'device_not_found'}}});
 await expect(preparePushAccount('other')).rejects.toBeTruthy();expect(api.revokePushDeviceProof).toHaveBeenCalledOnce();expect(deletePushToken).not.toHaveBeenCalled();
});
it('first login retires an unadopted foreign legacy token without transferring it', async () => {
 vi.mocked(api.revokeForeignLegacyToken).mockResolvedValue({cleanup_required:true});await preparePushAccount('other');
 expect(api.revokeForeignLegacyToken).toHaveBeenCalledWith('token');expect(deletePushToken).toHaveBeenCalledOnce();expect(api.enrollPushDevice).not.toHaveBeenCalled();
});
it('first login preserves same-owner legacy token for silent adoption', async () => {
 vi.mocked(api.revokeForeignLegacyToken).mockResolvedValue({cleanup_required:false});await preparePushAccount('member');expect(deletePushToken).not.toHaveBeenCalled();
 vi.mocked(api.getPushDevices).mockResolvedValue([]);vi.mocked(api.adoptPushDevice).mockImplementation(async input=>({...device,installation_id:input.installation_id}));
 expect(await restorePushDevice('member')).toBe(true);expect(api.enrollPushDevice).not.toHaveBeenCalled();
});
it('legacy cleanup failure persists token intent before revocation and retries after sign-in', async () => {
 vi.mocked(api.revokeForeignLegacyToken).mockImplementationOnce(async () => {
  expect(JSON.parse(values.get('rally:push-installation:v2')!).legacyCleanupToken).toBe('token');throw new Error('response lost');
 }).mockResolvedValueOnce({cleanup_required:true});
 await expect(preparePushAccount('other')).rejects.toThrow('response lost');await preparePushAccount('other');expect(deletePushToken).toHaveBeenCalledOnce();expect(JSON.parse(values.get('rally:push-installation:v2')!).legacyCleanupToken).toBeUndefined();
});
it('late cleanup response cannot act after a newer action clears the intent', async () => {
 seed();vi.mocked(api.revokePushDeviceProof).mockImplementationOnce(async () => {
  const current=JSON.parse(values.get('rally:push-installation:v2')!);current.cleanupIntent=undefined;values.set('rally:push-installation:v2',JSON.stringify(current));throw conflict(3);
 });
 await expect(preparePushAccount('other')).rejects.toThrow('newer action');expect(api.revokePushDeviceProof).toHaveBeenCalledOnce();expect(deletePushToken).not.toHaveBeenCalled();
});

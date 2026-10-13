import { api, type PushEnrollment } from './api';
import { deletePushToken, getExistingPushToken, requestNotificationPermission } from './firebase';

const key = 'rally:push-installation:v2';
const snapshots = new WeakMap<object, string>();
type LocalDevice = { owner: string; installation: string; secret: string; device?: PushEnrollment; disabled: boolean; operationID?: string; cleanupPending?: boolean; cleanupIntent?: string; cleanupGeneration?: number; registrationAttempted?: boolean; legacyCleanupToken?: string };
function read(): LocalDevice | null {
  const raw = window.localStorage.getItem(key);
  if (!raw) return null;
  const value = JSON.parse(raw) as LocalDevice;
  if (!value.installation || !value.secret || !value.owner) throw new Error('Device identity could not be loaded.');
  snapshots.set(value,raw);return value;
}
function currentOperation(value: LocalDevice, expected: string | null = value.operationID ?? null): void {
 const current = read();
 if (!current || current.owner !== value.owner || current.installation !== value.installation || current.secret !== value.secret || (current.operationID ?? null) !== expected || window.localStorage.getItem(key) !== snapshots.get(value)) {
  throw new Error('Device setup was replaced by a newer action.');
 }
}
function write(value: LocalDevice, expected: string | null = value.operationID ?? null) {
 currentOperation(value, expected);
 const serialized = JSON.stringify(value);
 if (window.localStorage.getItem(key) !== serialized) window.localStorage.setItem(key, serialized);
 snapshots.set(value,serialized);
}
function beginOperation(value: LocalDevice): LocalDevice {
 const previous = value.operationID ?? null;value.operationID = crypto.randomUUID();write(value, previous);return value;
}
function saveNewIdentity(value: LocalDevice, previous?: LocalDevice): void {
 if (previous) currentOperation(previous);
 else if (read()) throw new Error('Device setup was replaced by a newer action.');
 value.operationID = crypto.randomUUID();const serialized=JSON.stringify(value);window.localStorage.setItem(key,serialized);snapshots.set(value,serialized);
}
function identity(owner: string): LocalDevice {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return { owner, installation: crypto.randomUUID(), secret: Array.from(bytes, x => x.toString(16).padStart(2, '0')).join(''), disabled: false };
}
let localLock = Promise.resolve();
function serial<T>(work: () => Promise<T>): Promise<T> {
  if (navigator.locks) return navigator.locks.request('rally:push-device', work);
  const result = localLock.then(work, work); localLock = result.then(() => {}, () => {}); return result;
}
function isMissing(error: unknown): boolean { return (error as { response?: { status?: number } })?.response?.status === 404; }
function generationConflict(error: unknown): number | null {
 const data = (error as { response?: { status?: number; data?: { code?: string; details?: { current_generation?: number } } } })?.response;
 const counter = data?.data?.details?.current_generation;
 return data?.status === 409 && data.data?.code === 'device_generation_changed' && Number.isSafeInteger(counter) && counter! >= 1 ? counter! : null;
}
function stillCleaning(local: LocalDevice, intent: string) {
 currentOperation(local);
 const current = read();
 if (!current || current.owner !== local.owner || current.installation !== local.installation || current.cleanupIntent !== intent || current.operationID !== local.operationID) throw new Error('Device cleanup was replaced by a newer action.');
}
async function cleanupLegacy(local: LocalDevice) {
 const token = local.legacyCleanupToken || await getExistingPushToken();currentOperation(local);
 if (!token) return;
 // Save the address/intent before requesting revoke, so lost responses and
 // failed Firebase deletion can resume without forgetting the former device.
 local.legacyCleanupToken = token; write(local);
 const result = await api.revokeForeignLegacyToken(token);
 currentOperation(local);
 if (result.cleanup_required) { await deletePushToken(); currentOperation(local); }
 local.legacyCleanupToken = undefined; write(local);
}
async function cleanup(local: LocalDevice, owned = false) {
 if (!local.cleanupIntent) { local.cleanupIntent = crypto.randomUUID(); write(local); }
 const intent = local.cleanupIntent;
 if (local.device?.state !== 'revoked' && (local.device || local.registrationAttempted)) {
  let generation = local.cleanupGeneration ?? local.device?.generation ?? 0;
  let retired = false;
  for (let attempt = 0; attempt < 3; attempt++) {
   stillCleaning(local, intent);
   try {
    if (owned && local.device) await api.revokePushDevice(local.device.id, generation);
    else await api.revokePushDeviceProof(local.installation, local.secret, generation);
    retired = true; break;
   } catch (error) {
    stillCleaning(local, intent);
    if (isMissing(error) && !local.device) { retired = true; break; }
    if (owned && (error as { response?: { status?: number } })?.response?.status === 409) { owned = false; continue; }
    const current = generationConflict(error);
    if (current === null) throw error;
    generation = current; local.cleanupGeneration = current; write(local);
   }
  }
  if (!retired) throw new Error('Device registration kept changing. Retry cleanup.');
  stillCleaning(local, intent);
  if (local.device) local.device = { ...local.device, state: 'revoked', generation: generation + 1 };
  local.cleanupPending = true; local.disabled = true; write(local);
 }
 if (local.device || local.registrationAttempted || local.cleanupPending) {
  await deletePushToken();
 } else {
  // A pre-rollout browser may still hold a legacy token with no local v2 ID.
  await cleanupLegacy(local);
 }
 stillCleaning(local, intent);
 local.cleanupPending = false; local.cleanupIntent = undefined; local.cleanupGeneration = undefined; local.registrationAttempted = false; local.disabled = true; write(local);
}
// Ordinary sign-out deliberately does not invoke this. Account replacement is
// saved only after both server retirement and browser address cleanup succeed.
export function preparePushAccount(owner: string): Promise<void> {
 return serial(async () => {
  const local = read();
  if (local && local.owner === owner) {
   if (local.legacyCleanupToken) await cleanupLegacy(beginOperation(local));
   return;
  }
  if (local) {
   beginOperation(local);await cleanup(local);saveNewIdentity(identity(owner),local);return;
  }
  // Claim the absent local state before waiting for Firebase; another tab's
  // later identity or cleanup action must fence this sign-in's response.
  const recovery=identity(owner);saveNewIdentity(recovery);
  const token=await getExistingPushToken();currentOperation(recovery);
  if (!token) return;
  recovery.legacyCleanupToken=token;write(recovery);await cleanupLegacy(recovery);
 });
}
async function ownedLocal(owner: string): Promise<LocalDevice> {
 const previous=read();
 if(previous && previous.owner!==owner)throw new Error('Another account manages push on this browser. Sign in again to switch accounts.');
 if(previous)return beginOperation(previous);
 const fresh=identity(owner);saveNewIdentity(fresh);return fresh;
}
async function repairAdoptedDevice(local: LocalDevice): Promise<boolean> {
 currentOperation(local);
 if(local.device?.state!=='invalidated')return local.device?.state==='active';
 await deletePushToken();currentOperation(local);
 const token=await getExistingPushToken();currentOperation(local);if(!token)return false;
 local.device=await api.refreshPushDevice(local.device.id,{installation_id:local.installation,token,generation:local.device.generation});
 write(local);return local.device.state==='active';
}
export function restorePushDevice(owner: string): Promise<boolean> {
  return serial(async () => {
    const local = await ownedLocal(owner);
    if (local.cleanupIntent) { await cleanup(local); return false; }
    if (local.cleanupPending) { await deletePushToken(); currentOperation(local); local.cleanupPending = false; write(local); }
    if (local.disabled) return false;
    const devices = await api.getPushDevices();currentOperation(local);
    const device = devices.find(d => d.installation_id === local.installation);
    if (device) {
      local.device = device; write(local);
      if (device.state === 'revoked') { local.disabled = true; write(local); return false; }
      if (device.state === 'invalidated') { await deletePushToken(); currentOperation(local); }
      const token = await getExistingPushToken(); currentOperation(local); if (!token) return device.state === 'active';
      local.device = await api.refreshPushDevice(device.id, { installation_id: local.installation, token, generation: device.generation });
      write(local); return true;
    }
    // Legacy registration adoption is read/setup metadata only: an absent token
    // never becomes enrolled by startup. Existing permission is required.
    const token = await getExistingPushToken(); currentOperation(local); if (!token) return false;
    local.registrationAttempted = true; write(local);
    try {
      local.device = await api.adoptPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: 0 });
      write(local); return repairAdoptedDevice(local);
    } catch (error) { if (isMissing(error)) { local.registrationAttempted = false; write(local); return false; } throw error; }
  });
}
export function enablePushDevice(owner: string): Promise<boolean> {
  return serial(async () => {
    const previous = read();
    if (previous && previous.owner !== owner) { beginOperation(previous); await cleanup(previous); saveNewIdentity(identity(owner),previous); }
    const local = await ownedLocal(owner);
    if (local.cleanupIntent) await cleanup(local);
    if (local.cleanupPending) { await deletePushToken(); currentOperation(local); local.cleanupPending = false; write(local); }
    // Re-read server generation so explicit enable uses the latest tombstone.
    const devices = await api.getPushDevices();currentOperation(local);
    const device = devices.find(d => d.installation_id === local.installation);
    if (device?.state === 'invalidated') { await deletePushToken(); currentOperation(local); }
    const token = await requestNotificationPermission(); currentOperation(local); if (!token) return false;
    local.registrationAttempted = true; write(local);
    local.device = await api.enrollPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: device?.generation || 0, device_name: 'This browser' });
    local.disabled = false; write(local); return repairAdoptedDevice(local);
  });
}
export function disablePushDevice(owner: string): Promise<void> {
 return serial(async () => {
  const local = await ownedLocal(owner);
  const devices = await api.getPushDevices();currentOperation(local);
  const device = devices.find(d => d.installation_id === local.installation);
  if (device) local.device = device;
  await cleanup(local, true);
 });
}
export function isPushDeviceDisabled(owner: string): boolean {
  const local = read(); return !!local && local.owner === owner && local.disabled;
}

// Revision-only writes fence stale requests without making idle tabs continually
// refresh each other. Relevant device/owner/cleanup changes still refresh UI.
export function isDeviceSetupStorageChange(event: StorageEvent): boolean {
 if(event.key !== key)return false;
 try {
  const oldValue=event.oldValue ? JSON.parse(event.oldValue) : null;
  const newValue=event.newValue ? JSON.parse(event.newValue) : null;
  if(oldValue)delete oldValue.operationID;if(newValue)delete newValue.operationID;
  return JSON.stringify(oldValue)!==JSON.stringify(newValue);
 } catch {return true;}
}

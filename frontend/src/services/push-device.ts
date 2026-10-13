import { api, type PushEnrollment } from './api';
import { deletePushToken, getExistingPushToken, requestNotificationPermission } from './firebase';

const key = 'rally:push-installation:v2';
type LocalDevice = { owner: string; installation: string; secret: string; device?: PushEnrollment; disabled: boolean; cleanupPending?: boolean; cleanupIntent?: string; cleanupGeneration?: number; registrationAttempted?: boolean; legacyCleanupToken?: string };
function read(): LocalDevice | null {
  const raw = window.localStorage.getItem(key);
  if (!raw) return null;
  const value = JSON.parse(raw) as LocalDevice;
  if (!value.installation || !value.secret || !value.owner) throw new Error('Device identity could not be loaded.');
  return value;
}
function write(value: LocalDevice) {
  const serialized = JSON.stringify(value);
  if (window.localStorage.getItem(key) !== serialized) window.localStorage.setItem(key, serialized);
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
 const current = read();
 if (!current || current.owner !== local.owner || current.installation !== local.installation || current.cleanupIntent !== intent) throw new Error('Device cleanup was replaced by a newer action.');
}
async function cleanupLegacy(local: LocalDevice) {
 const token = local.legacyCleanupToken || await getExistingPushToken();
 if (!token) return;
 // Save the address/intent before requesting revoke, so lost responses and
 // failed Firebase deletion can resume without forgetting the former device.
 local.legacyCleanupToken = token; write(local);
 const result = await api.revokeForeignLegacyToken(token);
 if (result.cleanup_required) await deletePushToken();
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
   if (local.legacyCleanupToken) await cleanupLegacy(local);
   return;
  }
  if (local) { await cleanup(local); write(identity(owner)); return; }
  const token = await getExistingPushToken();
  if (!token) return;
  const recovery = identity(owner); recovery.legacyCleanupToken = token; write(recovery);
  await cleanupLegacy(recovery);
 });
}
async function ownedLocal(owner: string): Promise<LocalDevice> {
  const previous = read();
  if (previous && previous.owner !== owner) throw new Error('Another account manages push on this browser. Sign in again to switch accounts.');
  if (previous) return previous;
  const fresh = identity(owner); write(fresh); return fresh;
}
export function restorePushDevice(owner: string): Promise<boolean> {
  return serial(async () => {
    const local = await ownedLocal(owner);
    if (local.cleanupIntent) { await cleanup(local); return false; }
    if (local.cleanupPending) { await deletePushToken(); local.cleanupPending = false; write(local); }
    if (local.disabled) return false;
    const devices = await api.getPushDevices();
    const device = devices.find(d => d.installation_id === local.installation);
    if (device) {
      local.device = device; write(local);
      if (device.state === 'revoked') { local.disabled = true; write(local); return false; }
      if (device.state === 'invalidated') await deletePushToken();
      const token = await getExistingPushToken(); if (!token) return device.state === 'active';
      local.device = await api.refreshPushDevice(device.id, { installation_id: local.installation, token, generation: device.generation });
      write(local); return true;
    }
    // Legacy registration adoption is read/setup metadata only: an absent token
    // never becomes enrolled by startup. Existing permission is required.
    const token = await getExistingPushToken(); if (!token) return false;
    local.registrationAttempted = true; write(local);
    try {
      local.device = await api.adoptPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: 0 });
      write(local); return true;
    } catch (error) { if (isMissing(error)) { local.registrationAttempted = false; write(local); return false; } throw error; }
  });
}
export function enablePushDevice(owner: string): Promise<boolean> {
  return serial(async () => {
    const previous = read();
    if (previous && previous.owner !== owner) { await cleanup(previous); write(identity(owner)); }
    const local = await ownedLocal(owner);
    if (local.cleanupIntent) await cleanup(local);
    if (local.cleanupPending) { await deletePushToken(); local.cleanupPending = false; write(local); }
    // Re-read server generation so explicit enable uses the latest tombstone.
    const devices = await api.getPushDevices();
    const device = devices.find(d => d.installation_id === local.installation);
    const token = await requestNotificationPermission(); if (!token) return false;
    local.registrationAttempted = true; write(local);
    local.device = await api.enrollPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: device?.generation || 0, device_name: 'This browser' });
    local.disabled = false; write(local); return true;
  });
}
export function disablePushDevice(owner: string): Promise<void> {
 return serial(async () => {
  const local = await ownedLocal(owner);
  const devices = await api.getPushDevices();
  const device = devices.find(d => d.installation_id === local.installation);
  if (device) local.device = device;
  await cleanup(local, true);
 });
}
export function isPushDeviceDisabled(owner: string): boolean {
  const local = read(); return !!local && local.owner === owner && local.disabled;
}

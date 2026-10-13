import { api, type PushEnrollment } from './api';
import { deletePushToken, getExistingPushToken, requestNotificationPermission } from './firebase';

const key = 'rally:push-installation:v2';
type LocalDevice = { owner: string; installation: string; secret: string; device?: PushEnrollment; disabled: boolean; cleanupPending?: boolean };
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
async function cleanup(local: LocalDevice) {
  if (local.device && local.device.state !== 'revoked') {
    await api.revokePushDeviceProof(local.installation, local.secret, local.device.generation);
    local.device = { ...local.device, state: 'revoked', generation: local.device.generation + 1 };
    local.disabled = true; local.cleanupPending = true; write(local);
  }
  if (local.device || local.cleanupPending) await deletePushToken();
  local.cleanupPending = false; write(local);
}
// Ordinary sign-out deliberately does not call this. Only a different signed-in
// member retires the previous installation, using its revoke-only proof.
export function preparePushAccount(owner: string): Promise<void> {
  return serial(async () => {
    const local = read();
    if (!local || local.owner === owner) return;
    await cleanup(local);
    write(identity(owner));
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
    try {
      local.device = await api.adoptPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: 0 });
      write(local); return true;
    } catch (error) { if (isMissing(error)) return false; throw error; }
  });
}
export function enablePushDevice(owner: string): Promise<boolean> {
  return serial(async () => {
    const previous = read();
    if (previous && previous.owner !== owner) { await cleanup(previous); write(identity(owner)); }
    const local = await ownedLocal(owner);
    if (local.cleanupPending) { await deletePushToken(); local.cleanupPending = false; write(local); }
    // Re-read server generation so explicit enable uses the latest tombstone.
    const devices = await api.getPushDevices();
    const device = devices.find(d => d.installation_id === local.installation);
    const token = await requestNotificationPermission(); if (!token) return false;
    local.device = await api.enrollPushDevice({ installation_id: local.installation, revocation_secret: local.secret, token, generation: device?.generation || 0, device_name: 'This browser' });
    local.disabled = false; write(local); return true;
  });
}
export function disablePushDevice(owner: string): Promise<void> {
  return serial(async () => {
    const local = await ownedLocal(owner);
    const devices = await api.getPushDevices();
    const device = devices.find(d => d.installation_id === local.installation);
    if (device && device.state !== 'revoked') {
      await api.revokePushDevice(device.id, device.generation);
      local.device = { ...device, state: 'revoked', generation: device.generation + 1 };
    } else if (device) local.device = device;
    local.disabled = true; local.cleanupPending = true; write(local);
    await deletePushToken(); local.cleanupPending = false; write(local);
  });
}
export function isPushDeviceDisabled(owner: string): boolean {
  const local = read(); return !!local && local.owner === owner && local.disabled;
}

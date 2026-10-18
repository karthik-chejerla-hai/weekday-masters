type PendingAnnouncement = { requestId: string; payload: string };
const key = (actor: string) => `rally:announcement-request:${actor}`;
export function announcementRequest(actor: string, title: string, body: string, previous: PendingAnnouncement | null): PendingAnnouncement {
 const payload=JSON.stringify([title.trim(),body.trim()]);if(previous?.payload===payload)return previous;
 try {const stored=JSON.parse(sessionStorage.getItem(key(actor))||'null') as PendingAnnouncement|null;if(stored?.payload===payload&&typeof stored.requestId==='string')return stored;}catch{/* In-memory retries remain available. */}
 const pending={requestId:crypto.randomUUID(),payload};try{sessionStorage.setItem(key(actor),JSON.stringify(pending));}catch{/* Retain the component ref. */}return pending;
}
export function clearAnnouncementRequest(actor: string): void {try{sessionStorage.removeItem(key(actor));}catch{/* Publication already succeeded. */}}

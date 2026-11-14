import { beforeEach, expect, it, vi } from 'vitest';
import { announcementRequest, clearAnnouncementRequest } from './announcement-request';
const saved=new Map<string,string>();
beforeEach(()=>{saved.clear();vi.stubGlobal('sessionStorage',{getItem:(key:string)=>saved.get(key)||null,setItem:(key:string,value:string)=>saved.set(key,value),removeItem:(key:string)=>saved.delete(key)});});
it('retains identity across retry and reload for unchanged canonical content',()=>{
 const first=announcementRequest('admin',' News ',' Message ',null);
 expect(announcementRequest('admin','News','Message',first).requestId).toBe(first.requestId);
 expect(announcementRequest('admin','News','Message',null).requestId).toBe(first.requestId);
});
it('new content and a different admin get different request identities',()=>{
 const first=announcementRequest('admin','News','Message',null);
 expect(announcementRequest('admin','News','Changed',first).requestId).not.toBe(first.requestId);
 expect(announcementRequest('other-admin','News','Message',null).requestId).not.toBe(first.requestId);
});
it('success clears pending identity so an intentional later publication is new',()=>{
 const first=announcementRequest('admin','News','Message',null);clearAnnouncementRequest('admin');
 expect(announcementRequest('admin','News','Message',null).requestId).not.toBe(first.requestId);
});

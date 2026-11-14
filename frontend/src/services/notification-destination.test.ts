import { expect, it } from 'vitest';
import { notificationDestination, safeNotificationPath, historyDestination } from './notification-destination';
const id = '11111111-1111-4111-8111-111111111111';
it.each(['https://evil.test', '//evil.test', '/sessions/%2f%2fevil', '/notifications/garbage', '/money?url=evil', '/admin'])('rejects %s', path => expect(safeNotificationPath(path)).toBeNull());
it('routes receipts, legacy reminders, balances and exact announcements', () => {
 expect(notificationDestination({ type: 'session_charge', session_id: id })).toBe(`/sessions/${id}/settlement`);
 expect(notificationDestination({ type: 'rsvp_deadline', session_id: id })).toBe(`/sessions/${id}`);
 expect(notificationDestination({ type: 'balance_negative' })).toBe('/money');
 expect(historyDestination({ id, notification_type: 'admin_announcement', data: '{}' })).toBe(`/notifications/${id}`);
 expect(historyDestination({ id, notification_type: 'admin_announcement', data: 'null' })).toBe(`/notifications/${id}`);
 expect(notificationDestination({ destination_path: '//evil.test', session_id: id })).toBe(`/sessions/${id}`);
});
it('keeps historical receipt identity in safe routes and rejects extra query parameters', () => {
 const split = `/sessions/${id}/settlement?settlement_id=${id}`;
 expect(safeNotificationPath(split)).toBe(split);
 expect(notificationDestination({ type: 'session_charge_reversed', session_id: id, settlement_id: id })).toBe(split);
 expect(safeNotificationPath(`${split}&next=https://evil.test`)).toBeNull();
 expect(safeNotificationPath(`/sessions/${id}?settlement_id=${id}`)).toBeNull();
 expect(safeNotificationPath(`/sessions/${id}/settlement?settlement_id=bad`)).toBeNull();
});

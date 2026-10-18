// Keep these rules aligned with the push worker and server destination builder.
const id = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}';
const resource = new RegExp(`^/(?:sessions/${id}(?:/settlement(?:\\?settlement_id=${id})?)?|notifications/${id})$`);
const uuid = new RegExp(`^${id}$`);
export function safeNotificationPath(path: unknown): string | null {
  return typeof path === 'string' && (['/money', '/notifications', '/dashboard'].includes(path) || resource.test(path)) ? path : null;
}
export function notificationDestination(data: Record<string, string> = {}): string {
  const explicit = safeNotificationPath(data.destination_path);
  if (explicit) return explicit;
  if (uuid.test(data.session_id || '')) {
    const split = ['session_charge', 'session_charge_reversed'].includes(data.type);
    const settlement = split && uuid.test(data.settlement_id || '') ? `?settlement_id=${data.settlement_id}` : '';
    return `/sessions/${data.session_id}${split ? '/settlement' : ''}${settlement}`;
  }
  if (['balance_low', 'balance_negative'].includes(data.type)) return '/money';
  if (uuid.test(data.notification_id || '')) return `/notifications/${data.notification_id}`;
  return '/notifications';
}
export function historyDestination(item: { id: string; notification_type: string; data?: string; destination_path?: string }): string {
  let data: Record<string, string> = {};
  try { const parsed = JSON.parse(item.data || '{}'); if (parsed && typeof parsed === 'object') data = parsed; } catch { /* Legacy empty payload. */ }
  return notificationDestination({ ...data, type: item.notification_type, notification_id: item.id, destination_path: item.destination_path || '' });
}

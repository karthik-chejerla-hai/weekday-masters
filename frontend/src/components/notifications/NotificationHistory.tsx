import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../services/api';
import { historyDestination } from '../../services/notification-destination';
import type { Notification } from '../../services/api';

export default function NotificationHistory() {
  const [items, setItems] = useState<Notification[]>([]);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  useEffect(() => {
    let active = true;
    setLoading(true); setError(false);
    api.getNotificationHistory(20, offset).then(result => { if (active) setItems(result); })
      .catch(() => { if (active) setError(true); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [offset]);
  return <section className="space-y-4">
    <h1 className="text-xl font-semibold">Notifications</h1>
    <p className="text-sm text-slate-600">Your notification history. Saved messages do not confirm push delivery.</p>
    {loading ? <p role="status">Loading notifications…</p> : error ? <p role="alert">Could not load notifications. Please try again.</p> : items.length === 0 ? <p>No notifications on this page.</p> : <ul className="space-y-3">{items.map(item => <li key={item.id} className="card p-4">
      <Link className="block rounded focus:ring-2 focus:ring-primary-600" to={historyDestination(item)} onClick={() => { void api.markNotificationRead(item.id).catch(() => {}); }}>
        <h2 className="font-semibold">{item.title}</h2><p className="whitespace-pre-wrap text-slate-700">{item.body}</p>
        <time className="text-xs text-slate-500" dateTime={item.created_at}>{new Date(item.created_at).toLocaleString('en-AU', { timeZone: 'Australia/Sydney' })}</time>
      </Link>
    </li>)}</ul>}
    <div className="flex gap-3"><button className="btn-secondary" disabled={loading || offset === 0} onClick={() => setOffset(Math.max(0, offset - 20))}>Previous</button><button className="btn-secondary" disabled={loading || error || items.length < 20} onClick={() => setOffset(offset + 20)}>Next</button></div>
  </section>;
}

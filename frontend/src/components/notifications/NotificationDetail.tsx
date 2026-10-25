import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { api } from '../../services/api';
import { historyDestination } from '../../services/notification-destination';
import type { Notification } from '../../services/api';

export default function NotificationDetail() {
  const { id } = useParams();
  const [item, setItem] = useState<Notification | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let active = true; setItem(null); setError(null); setLoading(true);
    if (!id) { setError('Notification unavailable.'); setLoading(false); return; }
    api.getNotification(id).then(result => {
      if (active) { setItem(result); void api.markNotificationRead(result.id).catch(() => {}); }
    }).catch((e: { response?: { status?: number } }) => {
      if (active) setError(e.response?.status === 404 ? 'This notification is unavailable for your account.' : 'Could not load this notification. Please try again.');
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [id]);
  const destination = item ? historyDestination(item) : '';
  return <section className="space-y-4"><Link className="text-primary-700" to="/notifications">All notifications</Link>
    {loading ? <p role="status">Loading notification…</p> : error ? <p role="alert">{error}</p> : item && <article className="card p-5 space-y-3"><h1 className="text-xl font-semibold">{item.title}</h1><p className="whitespace-pre-wrap">{item.body}</p>{destination !== `/notifications/${id}` && <Link className="text-primary-700" to={destination}>View details</Link>}</article>}
  </section>;
}

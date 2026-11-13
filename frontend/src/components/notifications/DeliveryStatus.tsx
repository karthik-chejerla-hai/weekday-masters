import { useState } from 'react';
import { api } from '../../services/api';

const labels: Record<string,string> = {
 pending: 'Queued', claimed: 'Processing', sending: 'Processing',
 completed: 'Accepted by the push provider', accepted: 'Accepted by the push provider',
 partial: 'Partially accepted', uncertain: 'Provider acceptance uncertain',
 failed: 'Push failed', suppressed: 'Push suppressed', expired: 'Push expired',
 transient_failure: 'Waiting to retry',
};

export default function DeliveryStatus({ notificationId }: { notificationId: string }) {
 const [message,setMessage]=useState('');
 const [loading,setLoading]=useState(false);
 const [error,setError]=useState(false);
 async function refresh() {
  setLoading(true);setError(false);
  try {
   const result=await api.getNotificationDelivery(notificationId);
   const counts=Object.entries(result.devices).map(([state,count])=>`${count} ${labels[state] || state}`).join('; ');
   setMessage(`${labels[result.state] || result.state}${counts ? `: ${counts}` : ''}. Device receipt is not confirmed.`);
  } catch {setError(true);setMessage('Could not refresh push status. Try again.');}
  finally {setLoading(false);}
 }
 return <div className="mt-1 text-xs text-slate-600">
  <button type="button" className="underline min-h-9" disabled={loading} onClick={()=>void refresh()}>{loading ? 'Checking push status…' : 'Check push status'}</button>
  {message && <p role={error ? 'alert' : 'status'}>{message}</p>}
 </div>;
}

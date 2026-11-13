import { expect, it, vi } from 'vitest';
import { render,screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api } from '../../services/api';
import DeliveryStatus from './DeliveryStatus';
vi.mock('../../services/api',()=>({api:{getNotificationDelivery:vi.fn()}}));
it('refreshes queued work into an uncertain outcome without claiming receipt',async()=>{
 const user=userEvent.setup();
 vi.mocked(api.getNotificationDelivery).mockResolvedValueOnce({notification_id:'n',state:'pending',devices:{}}).mockResolvedValueOnce({notification_id:'n',state:'uncertain',devices:{uncertain:1}});
 render(<DeliveryStatus notificationId="n"/>);
 await user.click(screen.getByRole('button',{name:'Check push status'}));
 expect(await screen.findByRole('status')).toHaveTextContent('Queued');
 await user.click(screen.getByRole('button',{name:'Check push status'}));
 expect(await screen.findByRole('status')).toHaveTextContent('Provider acceptance uncertain');
 expect(screen.getByRole('status')).toHaveTextContent('Device receipt is not confirmed');
 expect(api.getNotificationDelivery).toHaveBeenLastCalledWith('n');
});
it('keeps a failed status lookup retryable',async()=>{
 vi.mocked(api.getNotificationDelivery).mockRejectedValueOnce(new Error('offline'));
 render(<DeliveryStatus notificationId="n"/>);
 await userEvent.click(screen.getByRole('button',{name:'Check push status'}));
 expect(await screen.findByRole('alert')).toHaveTextContent('Try again');
 expect(screen.getByRole('button',{name:'Check push status'})).toBeEnabled();
});

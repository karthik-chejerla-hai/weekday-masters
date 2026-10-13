import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import NotificationHistory from './NotificationHistory';
import NotificationDetail from './NotificationDetail';
import { api, type Notification } from '../../services/api';
import { Routes, Route } from 'react-router-dom';
vi.mock('../../services/api', () => ({ api: { getNotificationHistory: vi.fn(), getNotification: vi.fn(), markNotificationRead: vi.fn(async () => {}) } }));
beforeEach(() => vi.clearAllMocks());
const id = '11111111-1111-4111-8111-111111111111';
const notice = { id, notification_type: 'admin_announcement', title: 'Court change', body: 'Court four', data: '{}', created_at: '2026-10-13T00:00:00Z' } as Notification;
it('links the exact announcement from history', async () => {
 vi.mocked(api.getNotificationHistory).mockResolvedValue([notice]);
 render(<MemoryRouter><NotificationHistory /></MemoryRouter>);
 expect(await screen.findByRole('link', { name: /Court change/ })).toHaveAttribute('href', `/notifications/${id}`);
});
it('reports history failures separately from empty history', async () => {
 vi.mocked(api.getNotificationHistory).mockRejectedValue(new Error('offline'));
 render(<MemoryRouter><NotificationHistory /></MemoryRouter>);
 expect(await screen.findByRole('alert')).toHaveTextContent('Could not load');
 expect(screen.queryByText('No notifications on this page.')).not.toBeInTheDocument();
});
it('displays exact own event and marks it read', async () => {
 vi.mocked(api.getNotification).mockResolvedValue({ ...notice, destination_path: `/notifications/${id}` });
 render(<MemoryRouter initialEntries={[`/notifications/${id}`]}><Routes><Route path="/notifications/:id" element={<NotificationDetail />} /></Routes></MemoryRouter>);
 expect(await screen.findByRole('heading', { name: 'Court change' })).toBeInTheDocument();
 await waitFor(() => expect(api.markNotificationRead).toHaveBeenCalledWith(id));
});
it('handles foreign or deleted detail without exposing data', async () => {
 vi.mocked(api.getNotification).mockRejectedValue({ response: { status: 404 } });
 render(<MemoryRouter initialEntries={[`/notifications/${id}`]}><Routes><Route path="/notifications/:id" element={<NotificationDetail />} /></Routes></MemoryRouter>);
 expect(await screen.findByRole('alert')).toHaveTextContent('unavailable for your account');
 expect(api.markNotificationRead).not.toHaveBeenCalled();
});

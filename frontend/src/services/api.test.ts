import { describe, it, expect, vi, beforeEach } from 'vitest';

// The client is created at module load, so the mock has to be in place before
// api.ts is imported. vi.mock is hoisted, which is what makes this work.
const client = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  interceptors: { request: { use: vi.fn() } },
};

vi.mock('axios', () => ({
  default: { create: vi.fn(() => client) },
}));

const { api } = await import('./api');

// Captured at import time: the interceptor is registered in the constructor, and
// the clearAllMocks below would otherwise erase the call that recorded it.
const requestInterceptor = client.interceptors.request.use.mock.calls[0][0] as (
  config: { headers: Record<string, string> },
) => { headers: Record<string, string> };

// Every method here is a one-line axios call, so the thing worth asserting is
// the contract: the path, the verb and the body shape have to match the routes
// in cmd/server/main.go. A typo in a URL is invisible until runtime otherwise.

function resolvesTo<T>(value: T) {
  return Promise.resolve({ data: value });
}

beforeEach(() => {
  vi.clearAllMocks();
  client.get.mockReturnValue(resolvesTo(null));
  client.post.mockReturnValue(resolvesTo(null));
  client.put.mockReturnValue(resolvesTo(null));
  client.delete.mockReturnValue(resolvesTo(null));
});

describe('access token handling', () => {
  it('attaches a Bearer header once a token is set', () => {
    api.setAccessToken('token-123');
    expect(requestInterceptor({ headers: {} }).headers.Authorization).toBe('Bearer token-123');
  });

  it('sends no Authorization header when the token is cleared', () => {
    api.setAccessToken(null);
    expect(requestInterceptor({ headers: {} }).headers.Authorization).toBeUndefined();
  });
});

describe('request contracts', () => {
  it('posts only display fields to the auth callback', async () => {
    // Identity comes from the verified token on the backend; sending an email
    // or a subject from here would be a way to choose your own account.
    await api.authCallback('Jane', 'https://pic.example/j.png');

    expect(client.post).toHaveBeenCalledWith('/auth/callback', {
      name: 'Jane',
      profile_picture: 'https://pic.example/j.png',
    });
  });

  it.each([
    ['getMe', () => api.getMe(), 'get', ['/users/me']],
    ['listMembers', () => api.listMembers(), 'get', ['/users']],
    ['getClub', () => api.getClub(), 'get', ['/club']],
    ['listSessions', () => api.listSessions(), 'get', ['/sessions']],
    ['listCancelledSessions', () => api.listCancelledSessions(), 'get', ['/sessions/cancelled']],
    ['getSession', () => api.getSession('s1'), 'get', ['/sessions/s1']],
    ['listJoinRequests', () => api.listJoinRequests(), 'get', ['/admin/join-requests']],
    ['getNotificationPreferences', () => api.getNotificationPreferences(), 'get', ['/users/me/notifications']],
  ] as const)('%s issues the right GET', async (_name, call, method, args) => {
    await call();
    expect(client[method]).toHaveBeenCalledWith(...args);
  });

  it('sends the phone number under its snake_case key', async () => {
    await api.updateMe('+61412345678');
    expect(client.put).toHaveBeenCalledWith('/users/me', { phone_number: '+61412345678' });
  });

  it('posts and updates RSVPs against the session path', async () => {
    await api.createRSVP('s1', 'in');
    expect(client.post).toHaveBeenCalledWith('/sessions/s1/rsvp', { status: 'in' });

    await api.updateRSVP('s1', 'maybe');
    expect(client.put).toHaveBeenCalledWith('/sessions/s1/rsvp', { status: 'maybe' });

    await api.deleteRSVP('s1');
    expect(client.delete).toHaveBeenCalledWith('/sessions/s1/rsvp');
  });

  it('routes admin session management under /admin', async () => {
    await api.createSession({ title: 'Friday' } as never);
    expect(client.post).toHaveBeenCalledWith('/admin/sessions', { title: 'Friday' });

    await api.updateSession('s1', { courts: 3 } as never);
    expect(client.put).toHaveBeenCalledWith('/admin/sessions/s1', { courts: 3 });

    await api.deleteSession('s1');
    expect(client.delete).toHaveBeenCalledWith('/admin/sessions/s1');

    await api.cancelSession('s1', 'Flooded');
    expect(client.post).toHaveBeenCalledWith('/admin/sessions/s1/cancel', { reason: 'Flooded' });
  });

  it('puts the target user in the path for an admin RSVP', async () => {
    await api.adminAddRSVP('s1', 'u1', 'in');
    expect(client.post).toHaveBeenCalledWith('/admin/sessions/s1/rsvp/u1', { status: 'in' });
  });

  it('routes join request decisions and role changes', async () => {
    await api.approveJoinRequest('u1');
    expect(client.post).toHaveBeenCalledWith('/admin/join-requests/u1/approve');

    await api.rejectJoinRequest('u1');
    expect(client.post).toHaveBeenCalledWith('/admin/join-requests/u1/reject');

    await api.updateUserRole('u1', 'admin');
    expect(client.put).toHaveBeenCalledWith('/admin/users/u1/role', { role: 'admin' });
  });

  it('sends the push token in the body on register and delete', async () => {
    await api.registerPushToken('fcm-1', 'Pixel 8');
    expect(client.post).toHaveBeenCalledWith('/users/me/push-tokens', {
      token: 'fcm-1',
      device_name: 'Pixel 8',
    });

    // DELETE carries a body, which axios needs nested under `data`.
    await api.unregisterPushToken('fcm-1');
    expect(client.delete).toHaveBeenCalledWith('/users/me/push-tokens', {
      data: { token: 'fcm-1' },
    });
  });

  it('defaults notification history paging', async () => {
    await api.getNotificationHistory();
    expect(client.get).toHaveBeenCalledWith('/users/me/notifications/history', {
      params: { limit: 20, offset: 0 },
    });

    await api.getNotificationHistory(5, 10);
    expect(client.get).toHaveBeenCalledWith('/users/me/notifications/history', {
      params: { limit: 5, offset: 10 },
    });
  });

  it('routes the remaining notification and club calls', async () => {
    await api.markNotificationRead('n1');
    expect(client.post).toHaveBeenCalledWith('/notifications/n1/read');

    await api.updateNotificationPreferences({ push_enabled: false });
    expect(client.put).toHaveBeenCalledWith('/users/me/notifications', { push_enabled: false });

    await api.updateClub({ venue_name: 'New Courts' });
    expect(client.put).toHaveBeenCalledWith('/admin/club', { venue_name: 'New Courts' });

    await api.sendAnnouncement('Title', 'Body');
    expect(client.post).toHaveBeenCalledWith('/admin/announcements', {
      title: 'Title',
      body: 'Body',
    });
  });
});

describe('getMyRSVP', () => {
  it('returns the RSVP when one exists', async () => {
    client.get.mockReturnValue(resolvesTo({ id: 'r1', status: 'in' }));

    await expect(api.getMyRSVP('s1')).resolves.toEqual({ id: 'r1', status: 'in' });
    expect(client.get).toHaveBeenCalledWith('/sessions/s1/rsvp/me');
  });

  it('returns null rather than throwing when the member has not responded', async () => {
    // The backend answers 404 before a member RSVPs, and the callers treat that
    // as "no answer yet" rather than an error.
    client.get.mockRejectedValue(new Error('Request failed with status code 404'));

    await expect(api.getMyRSVP('s1')).resolves.toBeNull();
  });
});

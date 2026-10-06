import { test as base, expect, type Route } from '@playwright/test';
import type { Club, Session, SessionWithSummary, User } from '../../src/types';

export const NOW = '2026-10-07T01:00:00Z';
export const member: User = {
  id: 'member-1', auth0_id: 'auth0:browser-test', name: 'Alex Test', nickname: 'Alex',
  email: 'alex@example.invalid', phone_number: '', profile_picture: '', role: 'player',
  is_player: true, membership_status: 'approved', created_at: NOW, updated_at: NOW,
};
export const members = [member, ...['Blair', 'Casey', 'Drew'].map((name, index) => ({
  ...member, id: `member-${index + 2}`, name: `${name} Test`, nickname: name,
  auth0_id: `auth0:test-${index}`, email: `${name.toLowerCase()}@example.invalid`,
}))];
export const club: Club = {
  id: 'club-1', name: 'Rally Test Club', venue_name: 'Test Sports Hall',
  venue_address: '10 Test Street', time_format: '24h', created_at: NOW, updated_at: NOW,
};
export const session: Session = {
  id: 'session-1', title: 'Friday badminton', description: 'Weekly club game',
  session_date: '2026-10-16', start_time: '18:00', end_time: '20:00',
  starts_at: '2026-10-16T07:00:00Z', ends_at: '2026-10-16T09:00:00Z',
  courts: 1, max_players: 6, rsvp_deadline: '2026-10-13T12:59:59Z',
  is_recurring: false, recurring_day_of_week: null, recurring_parent_id: null,
  status: 'open', created_by: member.id, created_at: NOW, updated_at: NOW, rsvps: [],
};
export function detail(overrides: Partial<Session> = {}): SessionWithSummary {
  return { session: { ...structuredClone(session), ...overrides },
    rsvp_summary: { total_in: 0, total_out: 0, total_maybe: 0, total_waitlisted: 0,
      max_players: 6, spots_left: 6 } };
}
export const position = {
  assets: { bank_cents: 5000, court_credit_cents: 3000, shuttle_stock_cents: 2000,
    shuttle_stock_units: 12, total_cents: 10000 },
  liabilities: { player_balances_cents: 5000 }, surplus_cents: 5000, balanced: true, warnings: [],
};
export const preferences = {
  id: 'preferences-1', user_id: member.id, push_enabled: false, email_enabled: false,
  push_session_reminders: false, push_rsvp_deadlines: false, push_waitlist_updates: false,
  push_admin_announcements: false, push_balance_alerts: false, email_session_reminders: false,
  email_rsvp_deadlines: false, email_waitlist_updates: false,
  email_admin_announcements: false, email_balance_alerts: false, created_at: NOW, updated_at: NOW,
};
export async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
}

type Identity = 'visitor' | 'member' | 'admin' | 'pending' | 'removed';
type Fixtures = { identity: Identity; app: { user: User } };
export const test = base.extend<Fixtures>({
  identity: ['member', { option: true }],
  app: [async ({ page, context, identity }, use) => {
    const user: User = { ...member, role: identity === 'admin' ? 'admin' : 'player',
      membership_status: identity === 'pending' || identity === 'removed' ? identity : 'approved' };
    const unexpected: string[] = [];
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.clock.setFixedTime(new Date(NOW));
    await context.addInitScript((authenticated) => {
      // Do not overwrite logout state when the browser navigates or reloads.
      if (sessionStorage.getItem('e2e:initialized') === null) {
        sessionStorage.setItem('e2e:initialized', 'true');
        sessionStorage.setItem('e2e:authenticated', String(authenticated));
      }
    }, identity !== 'visitor');
    const defaults: Record<string, unknown> = {
      'GET /api/club': club,
      'POST /api/auth/callback': { user, is_new_user: false },
      'GET /api/users/me': user,
      'GET /api/users': members,
      'GET /api/assistant/status': { enabled: false },
      'GET /api/users/me/notifications': preferences,
      'GET /api/accounts/me': { balance_cents: 5000, state: 'ok' },
      'GET /api/accounts/me/spend': { year: 2026, as_of: '2026-10-07', recorded_from: null,
        ytd_cents: 0, all_time_cents: 0, months: [] },
      'GET /api/accounts': { items: [{ user_id: member.id, name: 'Alex', balance_cents: 5000 }] },
      'GET /api/accounts/activity': { items: [], total: 0 },
      'GET /api/position': position,
      'GET /api/sessions': [session],
      'GET /api/sessions/cancelled': [],
      'GET /api/sessions/unsettled': { items: [], total: 0 },
      'GET /api/sessions/session-1': detail(),
      'GET /api/sessions/session-1/games': { items: [], total: 0 },
    };
    await context.route('**/*', async route => {
      const url = new URL(route.request().url());
      if (url.origin !== 'http://127.0.0.1:4173') {
        unexpected.push(`External request: ${url.origin}${url.pathname}`);
        return route.abort();
      }
      if (!url.pathname.startsWith('/api/')) return route.continue();
      const key = `${route.request().method()} ${url.pathname}`;
      if (!(key in defaults)) {
        unexpected.push(key);
        return json(route, { error: `Unhandled test request: ${key}` }, 501);
      }
      if (url.pathname !== '/api/club') {
        expect(route.request().headers().authorization).toBe('Bearer browser-test-token');
      }
      await json(route, defaults[key]);
    });
    await use({ user });
    expect(unexpected, 'All network calls must have explicit test fixtures').toEqual([]);
    expect(errors, 'The application must not raise browser errors').toEqual([]);
  }, { auto: true }],
});
export { expect };

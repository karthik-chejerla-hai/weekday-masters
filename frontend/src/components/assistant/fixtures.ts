import type { ExpensePreview } from '../../types';

export function expenseFixture(): ExpensePreview {
  return {
    session: { id: 's1', title: 'Test session', starts_at: '2026-10-01T09:00:00Z', ends_at: '2026-10-01T11:00:00Z' },
    input: { total_hours: 3, shuttles_used: 8, participant_ids: ['u1', 'u2'], extra_participant_ids: ['u1', 'u2'] },
    fingerprint: 'a'.repeat(64), court_credit_after_cents: 3700, next_court_cost_cents: 6000, court_topup_needed: true,
    settlement: {
      bands: { base: { hours: 2, court_cents: 6000, shuttle_units: 8, shuttle_cents: 2222, total_cents: 8222, heads: 2 }, extra: { hours: 1, court_cents: 2300, shuttle_units: 0, shuttle_cents: 1111, total_cents: 3411, heads: 2 } },
      totals: { court_cents: 8300, shuttle_cents: 3333, shuttle_units: 8, charged_cents: 11633, surplus_cents: 0 },
      lines: [
        { user_id: 'u1', name: 'Priya', in_base: true, in_extra: true, comped: false, amount_cents: 5817 },
        { user_id: 'u2', name: 'Marcus', in_base: true, in_extra: true, comped: false, amount_cents: 5816 },
      ], stock_after: { units: 16, amount_cents: 6667 },
    },
  };
}

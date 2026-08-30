export type UserRole = 'pending' | 'player' | 'admin';
export type MembershipStatus = 'pending' | 'approved' | 'rejected';
export type RSVPStatus = 'in' | 'out' | 'maybe' | 'waitlisted';
/** Statuses a player may choose. "waitlisted" is assigned by the server. */
export type SelectableRSVPStatus = Exclude<RSVPStatus, 'waitlisted'>;
export type SessionStatus = 'open' | 'closed' | 'cancelled';

export interface User {
  id: string;
  auth0_id: string;
  email: string;
  name: string;
  profile_picture: string;
  phone_number: string;
  role: UserRole;
  is_player: boolean;
  membership_status: MembershipStatus;
  created_at: string;
  updated_at: string;
}

export interface Club {
  id: string;
  name: string;
  venue_name: string;
  venue_address: string;
  created_at: string;
  updated_at: string;
  // Settlement defaults. Present on the admin view of the club; the public
  // endpoint does not expose rates.
  base_hours?: number;
  base_rate_cents?: number;
  extra_rate_cents?: number;
  shuttles_per_hour?: number;
  low_balance_threshold_cents?: number;
}

export interface Session {
  id: string;
  title: string;
  description: string;
  session_date: string;
  start_time: string;
  end_time: string;
  courts: number;
  max_players: number;
  rsvp_deadline: string;
  is_recurring: boolean;
  recurring_day_of_week: number | null;
  recurring_parent_id: string | null;
  status: SessionStatus;
  cancellation_reason?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  rsvps?: RSVP[];
  creator?: User;
}

export interface RSVP {
  id: string;
  session_id: string;
  user_id: string;
  status: RSVPStatus;
  rsvp_timestamp: string;
  is_late_rsvp: boolean;
  added_by_admin: boolean;
  /** 1-based queue position; only present on waitlisted RSVPs. */
  waitlist_position?: number;
  created_at: string;
  updated_at: string;
  user?: User;
  session?: Session;
}

export interface RSVPSummary {
  total_in: number;
  total_out: number;
  total_maybe: number;
  total_waitlisted: number;
  max_players: number;
  spots_left: number;
}

export interface SessionWithSummary {
  session: Session;
  rsvp_summary: RSVPSummary;
}

export interface AuthCallbackResponse {
  user: User;
  is_new: boolean;
}

export interface CreateSessionInput {
  title: string;
  description?: string;
  session_date: string;
  start_time: string;
  end_time: string;
  courts: number;
  is_recurring?: boolean;
  recurring_day_of_week?: number;
  occurrences?: number;
  rsvp_deadline?: string; // RFC3339 datetime
}

export interface UpdateSessionInput {
  title?: string;
  description?: string;
  session_date?: string;
  start_time?: string;
  end_time?: string;
  courts?: number;
  status?: SessionStatus;
  rsvp_deadline?: string; // RFC3339 datetime
}

// --- Ledger ---------------------------------------------------------------
//
// All monetary values cross the wire as integer cents in `*_cents` fields and
// are only divided by 100 at the point of display. Nothing in the frontend
// does arithmetic on dollars.

export type AccountKind = 'player' | 'bank' | 'court_credit' | 'shuttle_stock' | 'surplus';

export type BalanceState = 'ok' | 'low' | 'negative';

export type TransactionKind =
  | 'player_topup'
  | 'withdrawal'
  | 'court_credit_purchase'
  | 'shuttle_purchase'
  | 'session_settlement'
  | 'opening_balance'
  | 'reversal';

export interface Account {
  id: string;
  kind: AccountKind;
  user_id?: string;
  name: string;
}

export interface PlayerBalance {
  user_id: string;
  name: string;
  profile_picture?: string;
  balance_cents: number;
}

export interface MyBalance {
  balance_cents: number;
  state: BalanceState;
}

export interface Transaction {
  id: string;
  kind: TransactionKind;
  session_id?: string;
  reverses_transaction_id?: string;
  description: string;
  occurred_at: string;
  created_by: string;
  created_at: string;
}

export interface LedgerEntryView {
  id: string;
  occurred_at: string;
  kind: TransactionKind;
  description: string;
  amount_cents: number;
  balance_after_cents: number;
  session_id?: string;
  reversed: boolean;
}

export interface ClubPosition {
  assets: {
    bank_cents: number;
    court_credit_cents: number;
    shuttle_stock_cents: number;
    shuttle_stock_units: number;
    total_cents: number;
  };
  liabilities: { player_balances_cents: number };
  surplus_cents: number;
  balanced: boolean;
  warnings: PositionWarning[];
}

export interface PositionWarning {
  code: string;
  message: string;
  next_session_id?: string;
}

// --- Settlement -----------------------------------------------------------

export interface SettlementBand {
  hours: number;
  court_cents: number;
  shuttle_units: number;
  shuttle_cents: number;
  total_cents: number;
  heads: number;
}

export interface ChargeLine {
  user_id: string;
  name?: string;
  guest_name?: string;
  in_base: boolean;
  in_extra: boolean;
  comped: boolean;
  amount_cents: number;
}

export interface SettlementRates {
  base_hours: number;
  base_rate_cents: number;
  extra_hours: number;
  extra_rate_cents: number;
  shuttles_per_hour: number;
}

export interface SettlementInput extends Partial<SettlementRates> {
  lines?: Array<Omit<ChargeLine, 'amount_cents' | 'name'>>;
}

export interface SettlementPreview {
  bands: { base: SettlementBand; extra: SettlementBand | null };
  totals: {
    court_cents: number;
    shuttle_cents: number;
    shuttle_units: number;
    charged_cents: number;
    surplus_cents: number;
  };
  lines: ChargeLine[];
  stock_after: { units: number; amount_cents: number };
}

export interface SettlementView extends SettlementPreview {
  session: { id: string; title: string; starts_at: string; ends_at: string };
  rates: SettlementRates;
  settled_at: string;
  reversed_at: string | null;
}

export interface PastSession {
  session_id: string;
  title: string;
  starts_at: string;
  ends_at: string;
  settled: boolean;
  total_cents: number;
  player_count: number;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export type UserRole = 'pending' | 'player' | 'admin';
export type MembershipStatus = 'pending' | 'approved' | 'rejected';
export type RSVPStatus = 'in' | 'out' | 'maybe';
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
  created_at: string;
  updated_at: string;
  user?: User;
  session?: Session;
}

export interface RSVPSummary {
  total_in: number;
  total_out: number;
  total_maybe: number;
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
}

export interface UpdateSessionInput {
  title?: string;
  description?: string;
  session_date?: string;
  start_time?: string;
  end_time?: string;
  courts?: number;
  status?: SessionStatus;
}

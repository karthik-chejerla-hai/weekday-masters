import axios, { AxiosInstance } from 'axios';
import type {
  User,
  Club,
  Session,
  RSVP,
  SessionWithSummary,
  AuthCallbackResponse,
  CreateSessionInput,
  UpdateSessionInput,
  SelectableRSVPStatus,
  PlayerBalance,
  MyBalance,
  LedgerEntryView,
  Transaction,
  SettlementInput,
  SettlementPreview,
  SettlementView,
  PastSession,
  ClubPosition,
} from '../types';

const API_URL = import.meta.env.VITE_API_URL || '/api';

class ApiService {
  private client: AxiosInstance;
  private accessToken: string | null = null;

  constructor() {
    this.client = axios.create({
      baseURL: API_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.client.interceptors.request.use((config) => {
      if (this.accessToken) {
        config.headers.Authorization = `Bearer ${this.accessToken}`;
      }
      return config;
    });
  }

  setAccessToken(token: string | null) {
    this.accessToken = token;
  }

  // Auth
  // The backend reads the Auth0 subject and email from the verified access token;
  // only display fields are sent here.
  async authCallback(name: string, profilePicture: string): Promise<AuthCallbackResponse> {
    const response = await this.client.post<AuthCallbackResponse>('/auth/callback', {
      name,
      profile_picture: profilePicture,
    });
    return response.data;
  }

  // Users
  async getMe(): Promise<User> {
    const response = await this.client.get<User>('/users/me');
    return response.data;
  }

  async updateMe(phoneNumber: string): Promise<User> {
    const response = await this.client.put<User>('/users/me', { phone_number: phoneNumber });
    return response.data;
  }

  async listMembers(): Promise<User[]> {
    const response = await this.client.get<User[]>('/users');
    return response.data;
  }

  // Club
  async getClub(): Promise<Club> {
    const response = await this.client.get<Club>('/club');
    return response.data;
  }

  // Sessions
  async listSessions(): Promise<Session[]> {
    const response = await this.client.get<Session[]>('/sessions');
    return response.data;
  }

  async listCancelledSessions(): Promise<Session[]> {
    const response = await this.client.get<Session[]>('/sessions/cancelled');
    return response.data;
  }

  async getSession(id: string): Promise<SessionWithSummary> {
    const response = await this.client.get<SessionWithSummary>(`/sessions/${id}`);
    return response.data;
  }

  // RSVPs
  async createRSVP(sessionId: string, status: SelectableRSVPStatus): Promise<RSVP> {
    const response = await this.client.post<RSVP>(`/sessions/${sessionId}/rsvp`, { status });
    return response.data;
  }

  async updateRSVP(sessionId: string, status: SelectableRSVPStatus): Promise<RSVP> {
    const response = await this.client.put<RSVP>(`/sessions/${sessionId}/rsvp`, { status });
    return response.data;
  }

  async deleteRSVP(sessionId: string): Promise<void> {
    await this.client.delete(`/sessions/${sessionId}/rsvp`);
  }

  async getMyRSVP(sessionId: string): Promise<RSVP | null> {
    try {
      const response = await this.client.get<RSVP>(`/sessions/${sessionId}/rsvp/me`);
      return response.data;
    } catch {
      return null;
    }
  }

  // Admin - Join Requests
  async listJoinRequests(): Promise<User[]> {
    const response = await this.client.get<User[]>('/admin/join-requests');
    return response.data;
  }

  async approveJoinRequest(userId: string): Promise<User> {
    const response = await this.client.post<User>(`/admin/join-requests/${userId}/approve`);
    return response.data;
  }

  async rejectJoinRequest(userId: string): Promise<User> {
    const response = await this.client.post<User>(`/admin/join-requests/${userId}/reject`);
    return response.data;
  }

  // Admin - User Management
  async updateUserRole(userId: string, role: string): Promise<User> {
    const response = await this.client.put<User>(`/admin/users/${userId}/role`, { role });
    return response.data;
  }

  // Admin - Sessions
  async createSession(input: CreateSessionInput): Promise<Session> {
    const response = await this.client.post<Session>('/admin/sessions', input);
    return response.data;
  }

  async updateSession(id: string, input: UpdateSessionInput): Promise<Session> {
    const response = await this.client.put<Session>(`/admin/sessions/${id}`, input);
    return response.data;
  }

  async deleteSession(id: string): Promise<void> {
    await this.client.delete(`/admin/sessions/${id}`);
  }

  async cancelSession(id: string, reason?: string): Promise<Session> {
    const response = await this.client.post<Session>(`/admin/sessions/${id}/cancel`, { reason });
    return response.data;
  }

  // Admin - RSVP Management
  async adminAddRSVP(sessionId: string, userId: string, status: SelectableRSVPStatus): Promise<RSVP> {
    const response = await this.client.post<RSVP>(`/admin/sessions/${sessionId}/rsvp/${userId}`, { status });
    return response.data;
  }

  // Admin - Club
  async updateClub(data: Partial<Club>): Promise<Club> {
    const response = await this.client.put<Club>('/admin/club', data);
    return response.data;
  }

  // Notifications - Preferences
  async getNotificationPreferences(): Promise<NotificationPreferences> {
    const response = await this.client.get<NotificationPreferences>('/users/me/notifications');
    return response.data;
  }

  async updateNotificationPreferences(updates: Partial<NotificationPreferences>): Promise<NotificationPreferences> {
    const response = await this.client.put<NotificationPreferences>('/users/me/notifications', updates);
    return response.data;
  }

  // Notifications - Push Tokens
  async registerPushToken(token: string, deviceName?: string): Promise<void> {
    await this.client.post('/users/me/push-tokens', { token, device_name: deviceName });
  }

  async unregisterPushToken(token?: string): Promise<void> {
    await this.client.delete('/users/me/push-tokens', { data: { token } });
  }

  // Notifications - History
  async getNotificationHistory(limit = 20, offset = 0): Promise<Notification[]> {
    const response = await this.client.get<Notification[]>('/users/me/notifications/history', {
      params: { limit, offset }
    });
    return response.data;
  }

  async markNotificationRead(notificationId: string): Promise<void> {
    await this.client.post(`/notifications/${notificationId}/read`);
  }

  // Admin - Announcements
  async sendAnnouncement(title: string, body: string): Promise<Announcement> {
    const response = await this.client.post<Announcement>('/admin/announcements', { title, body });
    return response.data;
  }

  // --- Ledger -------------------------------------------------------------
  //
  // Every amount here is integer cents. Nothing in the frontend does arithmetic
  // on dollars; formatCents divides by 100 at the point of display and nowhere
  // else.

  async listBalances(): Promise<PlayerBalance[]> {
    const response = await this.client.get<{ items: PlayerBalance[] }>('/accounts');
    return response.data.items ?? [];
  }

  async getMyBalance(): Promise<MyBalance> {
    const response = await this.client.get<MyBalance>('/accounts/me');
    return response.data;
  }

  async getMyEntries(limit = 50, offset = 0): Promise<{ items: LedgerEntryView[]; total: number }> {
    const response = await this.client.get<{ items: LedgerEntryView[]; total: number }>(
      '/accounts/me/entries',
      { params: { limit, offset } }
    );
    return { items: response.data.items ?? [], total: response.data.total ?? 0 };
  }

  async recordTopup(userId: string, amountCents: number, description?: string, occurredAt?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>('/admin/transactions/topup', {
      user_id: userId,
      amount_cents: amountCents,
      description,
      occurred_at: occurredAt,
    });
    return response.data;
  }

  async recordWithdrawal(userId: string, amountCents: number, description?: string, occurredAt?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>('/admin/transactions/withdrawal', {
      user_id: userId,
      amount_cents: amountCents,
      description,
      occurred_at: occurredAt,
    });
    return response.data;
  }

  async recordCourtCredit(amountCents: number, description?: string, occurredAt?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>('/admin/transactions/court-credit', {
      amount_cents: amountCents,
      description,
      occurred_at: occurredAt,
    });
    return response.data;
  }

  async recordShuttlePurchase(units: number, amountCents: number, description?: string, occurredAt?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>('/admin/transactions/shuttle-purchase', {
      units,
      amount_cents: amountCents,
      description,
      occurred_at: occurredAt,
    });
    return response.data;
  }

  async recordOpeningBalances(input: {
    players: Array<{ user_id: string; balance_cents: number }>;
    bank_cents: number;
    court_credit_cents: number;
    shuttle_stock: { units: number; amount_cents: number };
    occurred_at?: string;
  }): Promise<Transaction> {
    const response = await this.client.post<Transaction>('/admin/transactions/opening-balances', input);
    return response.data;
  }

  // The only way to undo anything. There is no edit or delete endpoint.
  async reverseTransaction(transactionId: string, description?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>(
      `/admin/transactions/${transactionId}/reverse`,
      { description }
    );
    return response.data;
  }

  // --- Settlement ---------------------------------------------------------

  /**
   * Cost a settlement without writing anything.
   *
   * Called on every change to the form, so the numbers on screen are always the
   * numbers that pressing settle will post.
   */
  async previewSettlement(sessionId: string, input: SettlementInput = {}): Promise<SettlementPreview> {
    const response = await this.client.post<SettlementPreview>(
      `/admin/sessions/${sessionId}/settlement/preview`,
      input
    );
    return response.data;
  }

  async settleSession(sessionId: string, input: SettlementInput = {}): Promise<SettlementPreview> {
    const response = await this.client.post<SettlementPreview>(
      `/admin/sessions/${sessionId}/settle`,
      input
    );
    return response.data;
  }

  async reverseSettlement(settlementId: string, description?: string): Promise<Transaction> {
    const response = await this.client.post<Transaction>(
      `/admin/settlements/${settlementId}/reverse`,
      { description }
    );
    return response.data;
  }

  async listSessionHistory(limit = 50, offset = 0): Promise<{ items: PastSession[]; total: number }> {
    const response = await this.client.get<{ items: PastSession[]; total: number }>(
      '/sessions/history',
      { params: { limit, offset } }
    );
    return { items: response.data.items ?? [], total: response.data.total ?? 0 };
  }

  async getSessionSettlement(sessionId: string): Promise<SettlementView> {
    const response = await this.client.get<SettlementView>(`/sessions/${sessionId}/settlement`);
    return response.data;
  }

  // --- Club position (admin) ----------------------------------------------

  async getClubPosition(): Promise<ClubPosition> {
    const response = await this.client.get<ClubPosition>('/admin/position');
    return response.data;
  }
}

// Notification types
export interface NotificationPreferences {
  id: string;
  user_id: string;
  push_enabled: boolean;
  push_session_reminders: boolean;
  push_rsvp_deadlines: boolean;
  push_waitlist_updates: boolean;
  push_admin_announcements: boolean;
  push_balance_alerts: boolean;
  email_enabled: boolean;
  email_session_reminders: boolean;
  email_rsvp_deadlines: boolean;
  email_waitlist_updates: boolean;
  email_admin_announcements: boolean;
  email_balance_alerts: boolean;
  created_at: string;
  updated_at: string;
}

export interface Notification {
  id: string;
  user_id: string;
  notification_type: string;
  title: string;
  body: string;
  data?: string;
  push_sent: boolean;
  push_sent_at?: string;
  email_sent: boolean;
  email_sent_at?: string;
  read_at?: string;
  created_at: string;
}

export interface Announcement {
  id: string;
  title: string;
  body: string;
  created_by: string;
  sent_at: string;
  created_at: string;
}

export const api = new ApiService();

import axios, { AxiosInstance } from 'axios';
import type {
  GameInput, GameResult, GameList, GameComparison, GamePlayer,
  AssistantMessage,
  AssistantReply,
  ExpenseInput,
  ExpensePreview,
  User,
  Club,
  InviteMemberInput,
  InvitationPreview,
  InvitationDelivery,
  UpdateMemberInput,
  UpdateProfileInput,
  Session,
  RSVP,
  SessionWithSummary,
  AuthCallbackResponse,
  CreateSessionInput,
  UpdateSessionInput,
  SelectableRSVPStatus,
  PlayerBalance,
  BalanceNudgeResult,
  MyBalance,
  PersonalSpend,
  LedgerEntryView,
  LedgerActivityView,
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

  /**
   * Updates the current member's own profile. Omitted fields are left
   * unchanged, so a save of one field cannot blank the other.
   */
  async updateMe(updates: UpdateProfileInput): Promise<User> {
    const response = await this.client.put<User>('/users/me', updates);
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

  async listGames(sessionId: string, offset = 0, signal?: AbortSignal): Promise<GameList> {
    return (await this.client.get<GameList>(`/sessions/${sessionId}/games`, { params: { offset }, signal })).data;
  }
  async createGame(sessionId: string, input: GameInput): Promise<GameResult> {
    return (await this.client.post<GameResult>(`/sessions/${sessionId}/games`, input)).data;
  }
  async updateGame(id: string, input: GameInput): Promise<GameResult> {
    return (await this.client.put<GameResult>(`/games/${id}`, input)).data;
  }
  async voidGame(id: string, version: number): Promise<GameResult> {
    return (await this.client.delete<GameResult>(`/games/${id}`, { data: { version } })).data;
  }
  async gameRevisions(id: string): Promise<GameResult[]> {
    return (await this.client.get<GameResult[]>(`/games/${id}/revisions`)).data;
  }
  async gamePlayers(): Promise<GamePlayer[]> {
    return (await this.client.get<GamePlayer[]>('/games/players')).data;
  }
  async headToHead(a: string[], b: string[], offset = 0, signal?: AbortSignal): Promise<GameComparison> {
    return (await this.client.get<GameComparison>('/games/head-to-head', { params: { team_a: a.join(','), team_b: b.join(','), offset }, signal })).data;
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

  // Admin - Member Management
  //
  // Note the two member lists: `listMembers` is the club's roll of approved
  // members, `adminListMembers` also carries the pending, rejected and removed
  // rows an admin acts on.
  async adminListMembers(): Promise<User[]> {
    const response = await this.client.get<User[]>('/admin/users');
    return response.data;
  }

  async inviteMember(input: InviteMemberInput): Promise<User> {
    const response = await this.client.post<User>('/admin/users', input);
    return response.data;
  }

  async previewInvitation(userId: string): Promise<InvitationPreview> {
    return (await this.client.get<InvitationPreview>(`/admin/users/${userId}/invitation`)).data;
  }

  async sendInvitation(userId: string, requestId: string, expectedEmail: string, test = false): Promise<InvitationDelivery> {
    return (await this.client.post<InvitationDelivery>(`/admin/users/${userId}/invitation${test ? '/test' : ''}`, {
      request_id: requestId, expected_email: expectedEmail,
    })).data;
  }

  async updateMember(userId: string, input: UpdateMemberInput): Promise<User> {
    const response = await this.client.put<User>(`/admin/users/${userId}`, input);
    return response.data;
  }

  /** Revokes access. Not a delete — `reinstateMember` undoes it. */
  async removeMember(userId: string): Promise<User> {
    const response = await this.client.delete<User>(`/admin/users/${userId}`);
    return response.data;
  }

  async reinstateMember(userId: string): Promise<User> {
    const response = await this.client.post<User>(`/admin/users/${userId}/reinstate`);
    return response.data;
  }

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
  async getMemberPushStatus(userId: string): Promise<MemberPushStatus> {
    const response = await this.client.get<MemberPushStatus>(`/admin/users/${userId}/push-notifications`);
    return response.data;
  }

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

  async nudgeBalance(userId: string): Promise<BalanceNudgeResult> {
    const response = await this.client.post<BalanceNudgeResult>(`/admin/users/${userId}/balance-nudge`);
    return response.data;
  }

  async getMyBalance(): Promise<MyBalance> {
    const response = await this.client.get<MyBalance>('/accounts/me');
    return response.data;
  }

  async getMySpend(): Promise<PersonalSpend> {
    const response = await this.client.get<PersonalSpend>('/accounts/me/spend');
    return response.data;
  }

  async getMyEntries(limit = 50, offset = 0): Promise<{ items: LedgerEntryView[]; total: number }> {
    const response = await this.client.get<{ items: LedgerEntryView[]; total: number }>(
      '/accounts/me/entries',
      { params: { limit, offset } }
    );
    return { items: response.data.items ?? [], total: response.data.total ?? 0 };
  }

  async getLedgerActivity(scope: 'mine' | 'all', topupsOnly: boolean, limit = 50, offset = 0): Promise<{ items: LedgerActivityView[]; total: number }> {
    const response = await this.client.get<{ items: LedgerActivityView[]; total: number }>(
      '/accounts/activity',
      { params: { scope, type: topupsOnly ? 'topup' : 'all', limit, offset } }
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

  async listUnsettledSessions(): Promise<{ items: PastSession[]; total: number }> {
    const response = await this.client.get('/sessions/unsettled');
    return response.data;
  }

  async previewExpense(sessionId: string, input: ExpenseInput): Promise<ExpensePreview> {
    const response = await this.client.post<ExpensePreview>(`/admin/sessions/${sessionId}/expense/preview`, input);
    return response.data;
  }

  async confirmExpense(sessionId: string, input: ExpenseInput): Promise<{ id: string }> {
    const response = await this.client.post(`/admin/sessions/${sessionId}/expense`, input);
    return response.data;
  }

  async assistantStatus(): Promise<{ enabled: boolean }> {
    const response = await this.client.get('/assistant/status');
    return response.data;
  }

  async askAssistant(messages: AssistantMessage[], sessionId?: string, signal?: AbortSignal): Promise<AssistantReply> {
    const response = await this.client.post<AssistantReply>('/assistant/messages', { messages, session_id: sessionId }, { signal });
    return response.data;
  }

  async transcribeAudio(audio: Blob, signal?: AbortSignal): Promise<{ text: string }> {
    const type = audio.type.split(';')[0];
    const extension = type.includes('mp4') ? 'mp4' : type.includes('ogg') ? 'ogg' : 'webm';
    const form = new FormData();
    form.append('file', audio, `recording.${extension}`);
    const response = await this.client.post('/assistant/transcribe', form, {
      headers: { 'Content-Type': undefined }, signal,
    });
    return response.data;
  }

  // --- Club position (approved members) -----------------------------------

  async getClubPosition(): Promise<ClubPosition> {
    const response = await this.client.get<ClubPosition>('/position');
    return response.data;
  }
}

export interface MemberPushStatus {
  preferences: NotificationPreferences | null;
  devices: Array<{
    id: string;
    device_name: string;
    created_at: string;
    last_registered_at: string;
  }>;
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

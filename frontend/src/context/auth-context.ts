import { createContext } from 'react';
import type { User } from '../types';

export interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isApproved: boolean;
  /** Effective role for the current view. False while an admin previews the member UI. */
  isAdmin: boolean;
  isViewingAsMember: boolean;
  startMemberPreview: () => void;
  stopMemberPreview: () => void;
  login: () => void;
  loginForInvitation?: () => void;
  authError?: string | null;
  retryAuth?: () => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
}

// Kept apart from AuthProvider so the provider module exports only a component,
// which is what lets React Fast Refresh reload it without dropping auth state.
export const AuthContext = createContext<AuthContextType | undefined>(undefined);

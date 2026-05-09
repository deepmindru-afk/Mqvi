/**
 * Auth Store — User session management.
 */

import { create } from "zustand";
import * as authApi from "../api/auth";
import { setTokens, clearTokens } from "../api/client";
import { changeLanguage, type Language, SUPPORTED_LANGUAGES } from "../i18n";
import { useE2EEStore } from "./e2eeStore";
import { usePreferencesStore } from "./preferencesStore";
import { useVoiceStore } from "./voiceStore";
import { useSettingsStore } from "./settingsStore";
import type { User, UserStatus } from "../types";

const MANUAL_STATUS_KEY = "mqvi_manual_status";

/** Apply user's DB language preference to i18n (takes priority over browser locale). */
function syncLanguageFromUser(user: User): void {
  if (user.language && user.language in SUPPORTED_LANGUAGES) {
    changeLanguage(user.language as Language);
  }
}

/**
 * AccountDeletedInfo — populated when login is attempted on a soft-deleted account.
 * Frontend reads this to show the recovery modal.
 */
export type AccountDeletedInfo = {
  username: string;
  deletedAt: string;
  permanentDeleteAt: string;
  /** The password the user just typed in the login form, reused for restore. */
  password: string;
};

type AuthState = {
  user: User | null;
  isLoading: boolean;
  error: string | null;
  isInitialized: boolean;
  accountDeleted: AccountDeletedInfo | null;

  // ─── Actions ───
  register: (username: string, password: string, displayName?: string, email?: string) => Promise<boolean>;
  login: (username: string, password: string) => Promise<boolean>;
  /** Restore a soft-deleted account using captured username + password. Returns true on success. */
  restoreAccount: () => Promise<boolean>;
  /** Dismiss the account-deleted recovery prompt without restoring. */
  cancelAccountDeleted: () => void;
  logout: () => Promise<void>;
  initialize: () => Promise<void>;
  clearError: () => void;
  updateUser: (partial: Partial<User>) => void;

  /**
   * User's manually selected presence. When set to "online", idle detection works normally.
   * When "dnd"/"idle"/"offline" (invisible), idle detection is disabled to preserve the choice.
   * Persisted in DB (pref_status column). localStorage is a local cache for UI before WS connects.
   * Authoritative value comes from server via ready event.
   */
  manualStatus: UserStatus;
  setManualStatus: (status: UserStatus) => void;
};

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: false,
  error: null,
  isInitialized: false,
  accountDeleted: null,
  manualStatus: (localStorage.getItem(MANUAL_STATUS_KEY) as UserStatus) || "online",

  register: async (username, password, displayName, email) => {
    set({ isLoading: true, error: null });

    const res = await authApi.register({
      username,
      password,
      display_name: displayName,
      email: email || undefined,
    });

    if (res.success && res.data) {
      setTokens(res.data.access_token, res.data.refresh_token);
      syncLanguageFromUser(res.data.user);
      set({ user: res.data.user, isLoading: false });
      usePreferencesStore.getState().fetchAndApply();
      return true;
    }

    set({ error: res.error ?? "Registration failed", isLoading: false });
    return false;
  },

  login: async (username, password) => {
    set({ isLoading: true, error: null, accountDeleted: null });

    const res = await authApi.login({ username, password });

    if (res.success && res.data) {
      setTokens(res.data.access_token, res.data.refresh_token);
      syncLanguageFromUser(res.data.user);
      set({ user: res.data.user, isLoading: false });
      // Fetch server-side preferences and apply to stores
      usePreferencesStore.getState().fetchAndApply();
      return true;
    }

    // Soft-deleted account: backend returns { success: false, error: "account_deleted",
    // data: { username, deleted_at, permanent_delete_at } } with HTTP 403.
    if (res.error === "account_deleted" && res.data) {
      const info = res.data as unknown as {
        username: string;
        deleted_at: string;
        permanent_delete_at: string;
      };
      set({
        accountDeleted: {
          username: info.username,
          deletedAt: info.deleted_at,
          permanentDeleteAt: info.permanent_delete_at,
          password,
        },
        isLoading: false,
        error: null,
      });
      return false;
    }

    set({ error: res.error ?? "Login failed", isLoading: false });
    return false;
  },

  restoreAccount: async () => {
    const info = get().accountDeleted;
    if (!info) return false;

    set({ isLoading: true, error: null });
    const res = await authApi.restoreAccount(info.username, info.password);

    if (res.success && res.data) {
      setTokens(res.data.access_token, res.data.refresh_token);
      syncLanguageFromUser(res.data.user);
      set({ user: res.data.user, isLoading: false, accountDeleted: null });
      usePreferencesStore.getState().fetchAndApply();
      return true;
    }

    set({ error: res.error ?? "Restore failed", isLoading: false });
    return false;
  },

  cancelAccountDeleted: () => set({ accountDeleted: null }),

  logout: async () => {
    // Leave voice channel first
    const voiceState = useVoiceStore.getState();
    if (voiceState.currentVoiceChannelId) {
      if (voiceState._onLeaveCallback) {
        voiceState._onLeaveCallback();
      } else {
        voiceState.leaveVoiceChannel();
      }
    }

    // Reset E2EE state (IndexedDB keys preserved)
    await useE2EEStore.getState().reset();
    usePreferencesStore.getState().reset();

    const refreshToken = localStorage.getItem("refresh_token");
    if (refreshToken) {
      await authApi.logout(refreshToken);
    }
    clearTokens();
    // Close settings modal if open (SPA doesn't reload between logout → login)
    useSettingsStore.getState().closeSettings();
    set({ user: null });
  },

  /** Restore session from stored token on app start. */
  initialize: async () => {
    const token = localStorage.getItem("access_token");
    if (!token) {
      set({ isInitialized: true });
      return;
    }
    // Persisted token must be re-pushed to Electron main on every cold start.
    void window.electronAPI?.setFileAuthToken(token);

    const res = await authApi.getMe();
    if (res.success && res.data) {
      syncLanguageFromUser(res.data);
      set({ user: res.data, isInitialized: true });
      usePreferencesStore.getState().fetchAndApply();
    } else {
      clearTokens();
      set({ isInitialized: true });
    }
  },

  clearError: () => set({ error: null }),

  updateUser: (partial) =>
    set((state) => ({
      user: state.user ? { ...state.user, ...partial } : null,
    })),

  setManualStatus: (status) => {
    localStorage.setItem(MANUAL_STATUS_KEY, status);
    set((state) => ({
      manualStatus: status,
      user: state.user ? { ...state.user, status } : null,
    }));
  },
}));

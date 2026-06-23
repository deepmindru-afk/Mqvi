/**
 * Push token cache — the device's current FCM token, mirrored in localStorage so
 * logout can unregister it. Kept separate from usePushNotifications so non-UI code
 * (authStore) can import the unregister helpers without pulling in React or the
 * navigation stores.
 */

import { registerPushToken, unregisterPushToken } from "../api/push";
import { getCapacitorPlatform } from "./constants";

const PUSH_TOKEN_KEY = "mqvi_push_token";

/** Caches the FCM token locally and registers it with the backend. */
export async function syncPushToken(value: string): Promise<void> {
  const platform = getCapacitorPlatform();
  if (platform !== "android" && platform !== "ios") return;
  localStorage.setItem(PUSH_TOKEN_KEY, value);
  const res = await registerPushToken({ token: value, platform });
  if (!res.success) {
    console.error("[push] failed to register token:", res.error);
  }
}

/** Removes this device's push token from the backend. Called on explicit logout. */
export async function unregisterCurrentPushToken(): Promise<void> {
  const token = localStorage.getItem(PUSH_TOKEN_KEY);
  if (!token) return;
  try {
    await unregisterPushToken(token);
  } finally {
    localStorage.removeItem(PUSH_TOKEN_KEY);
  }
}

/**
 * Clears only the local token cache (no server call). Used when a session restore
 * fails: the access token is already invalid so we can't unregister server-side.
 * The server row self-heals on next login (token upsert reassigns user_id); a
 * server-side prune on session revoke is the complete fix (tracked for a later phase).
 */
export function clearCachedPushToken(): void {
  localStorage.removeItem(PUSH_TOKEN_KEY);
}

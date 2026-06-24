/**
 * Push token cache — the device's current FCM token (and, on iOS, the PushKit VoIP
 * token) mirrored in localStorage so logout can unregister them. Kept separate from
 * the hooks so non-UI code (authStore) can import the unregister helpers without
 * pulling in React or the navigation stores.
 */

import { registerPushToken, unregisterPushToken } from "../api/push";
import { getCapacitorPlatform } from "./constants";

const PUSH_TOKEN_KEY = "mqvi_push_token";
const VOIP_TOKEN_KEY = "mqvi_voip_token";

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

/** Caches the iOS PushKit VoIP token (registered from useCallKit, a separate token
 * from the FCM one) so logout can unregister it too. */
export function cacheVoipToken(token: string): void {
  if (token) localStorage.setItem(VOIP_TOKEN_KEY, token);
}

/** Removes this device's push tokens (FCM + VoIP) from the backend. Called on logout. */
export async function unregisterCurrentPushToken(): Promise<void> {
  const fcm = localStorage.getItem(PUSH_TOKEN_KEY);
  const voip = localStorage.getItem(VOIP_TOKEN_KEY);
  try {
    if (fcm) await unregisterPushToken(fcm);
    if (voip) await unregisterPushToken(voip);
  } finally {
    localStorage.removeItem(PUSH_TOKEN_KEY);
    localStorage.removeItem(VOIP_TOKEN_KEY);
  }
}

/**
 * Clears only the local token caches (no server call). Used when a session restore
 * fails: the access token is already invalid so we can't unregister server-side.
 * The server row self-heals on next login (token upsert reassigns user_id); a
 * server-side prune on session revoke is the complete fix (tracked for a later phase).
 */
export function clearCachedPushToken(): void {
  localStorage.removeItem(PUSH_TOKEN_KEY);
  localStorage.removeItem(VOIP_TOKEN_KEY);
}

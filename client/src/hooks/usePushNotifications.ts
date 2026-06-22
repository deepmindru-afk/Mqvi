/**
 * usePushNotifications — registers the device for FCM push notifications.
 *
 * Capacitor (mobile) only; a no-op on web and Electron. Called once from
 * AppLayout while authenticated. The FCM token is cached in localStorage so
 * logout can unregister it from the backend.
 */

import { useEffect } from "react";
import { PushNotifications } from "@capacitor/push-notifications";
import type { PluginListenerHandle } from "@capacitor/core";

import { isCapacitor, getCapacitorPlatform } from "../utils/constants";
import { registerPushToken, unregisterPushToken } from "../api/push";

const PUSH_TOKEN_KEY = "mqvi_push_token";

async function syncToken(value: string): Promise<void> {
  const platform = getCapacitorPlatform();
  if (platform !== "android" && platform !== "ios") return;
  localStorage.setItem(PUSH_TOKEN_KEY, value);
  const res = await registerPushToken({ token: value, platform });
  if (!res.success) {
    console.error("[push] failed to register token:", res.error);
  }
}

/** Removes this device's push token from the backend. Called on logout. */
export async function unregisterCurrentPushToken(): Promise<void> {
  const token = localStorage.getItem(PUSH_TOKEN_KEY);
  if (!token) return;
  try {
    await unregisterPushToken(token);
  } finally {
    localStorage.removeItem(PUSH_TOKEN_KEY);
  }
}

export function usePushNotifications(): void {
  useEffect(() => {
    if (!isCapacitor()) return;

    const handles: PluginListenerHandle[] = [];
    let cancelled = false;

    async function setup(): Promise<void> {
      const perm = await PushNotifications.requestPermissions();
      if (perm.receive !== "granted") {
        console.warn("[push] permission not granted:", perm.receive);
        return;
      }
      if (cancelled) return;

      handles.push(
        await PushNotifications.addListener("registration", (token) => {
          void syncToken(token.value);
        }),
      );
      handles.push(
        await PushNotifications.addListener("registrationError", (err) => {
          console.error("[push] registration error:", err.error);
        }),
      );

      await PushNotifications.register();
    }

    void setup();

    return () => {
      cancelled = true;
      handles.forEach((h) => void h.remove());
    };
  }, []);
}

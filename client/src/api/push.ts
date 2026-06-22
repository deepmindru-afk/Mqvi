/**
 * Push notifications API — register/unregister this device's FCM token.
 */

import { apiClient } from "./client";

export function registerPushToken(req: {
  token: string;
  platform: "android" | "ios" | "web";
  device_label?: string;
}) {
  return apiClient<{ id: string }>("/push/tokens", {
    method: "POST",
    body: req,
  });
}

export function unregisterPushToken(token: string) {
  return apiClient<null>("/push/tokens", {
    method: "DELETE",
    body: { token },
  });
}

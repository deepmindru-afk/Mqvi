/**
 * P2PCall — native iOS PushKit/CallKit bridge (see ios/App/App/P2PCallPlugin.swift).
 * iOS-only; on other platforms the proxy methods are unused (callers guard on platform).
 */

import { registerPlugin } from "@capacitor/core";
import type { PluginListenerHandle } from "@capacitor/core";

export interface P2PCallPlugin {
  /** Current VoIP (PushKit) token, "" if not yet available. */
  getVoipToken(): Promise<{ token: string }>;
  /** Dismiss the CallKit UI when the call ends/declines in-app. */
  endCall(options: { call_id: string }): Promise<void>;

  addListener(
    eventName: "voipToken",
    listener: (data: { token: string }) => void,
  ): Promise<PluginListenerHandle>;
  addListener(
    eventName: "callAnswered",
    listener: (data: { call_id: string }) => void,
  ): Promise<PluginListenerHandle>;
  addListener(
    eventName: "callEnded",
    listener: (data: { call_id: string }) => void,
  ): Promise<PluginListenerHandle>;
}

export const P2PCall = registerPlugin<P2PCallPlugin>("P2PCall");

/**
 * voiceMessageStore — ephemeral voice channel chat messages, scoped per channel.
 *
 * Lifecycle is server-driven:
 *  - load via listVoiceMessages on chat panel open
 *  - real-time mutations via WS handlers (create/update/delete)
 *  - wipeChannel called when voice_channel_timer_stop arrives (last person left)
 */

import { create } from "zustand";
import type { VoiceMessage } from "../types";

type VoiceMessageState = {
  /** channelId -> messages (chronological asc) */
  messagesByChannel: Record<string, VoiceMessage[]>;

  /** Replace the entire list for a channel (used after listVoiceMessages). */
  setForChannel: (channelId: string, messages: VoiceMessage[]) => void;
  /** Append a single message from WS create event (dedup by id). */
  append: (message: VoiceMessage) => void;
  /** Replace an existing message in place after edit. */
  update: (message: VoiceMessage) => void;
  /** Remove a deleted message. */
  remove: (channelId: string, messageId: string) => void;
  /** Wipe all messages for a channel (fired when channel goes empty). */
  wipeChannel: (channelId: string) => void;
};

export const useVoiceMessageStore = create<VoiceMessageState>((set) => ({
  messagesByChannel: {},

  setForChannel: (channelId, messages) => {
    set((state) => ({
      messagesByChannel: { ...state.messagesByChannel, [channelId]: messages },
    }));
  },

  append: (message) => {
    set((state) => {
      const existing = state.messagesByChannel[message.channel_id] ?? [];
      if (existing.some((m) => m.id === message.id)) return state;
      return {
        messagesByChannel: {
          ...state.messagesByChannel,
          [message.channel_id]: [...existing, message],
        },
      };
    });
  },

  update: (message) => {
    set((state) => {
      const list = state.messagesByChannel[message.channel_id];
      if (!list) return state;
      let changed = false;
      const next = list.map((m) => {
        if (m.id === message.id) {
          changed = true;
          return message;
        }
        return m;
      });
      if (!changed) return state;
      return {
        messagesByChannel: { ...state.messagesByChannel, [message.channel_id]: next },
      };
    });
  },

  remove: (channelId, messageId) => {
    set((state) => {
      const list = state.messagesByChannel[channelId];
      if (!list) return state;
      const next = list.filter((m) => m.id !== messageId);
      if (next.length === list.length) return state;
      return {
        messagesByChannel: { ...state.messagesByChannel, [channelId]: next },
      };
    });
  },

  wipeChannel: (channelId) => {
    set((state) => {
      if (!(channelId in state.messagesByChannel)) return state;
      const next = { ...state.messagesByChannel };
      delete next[channelId];
      return { messagesByChannel: next };
    });
  },
}));

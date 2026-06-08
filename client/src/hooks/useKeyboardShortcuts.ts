/**
 * useKeyboardShortcuts — Global keyboard shortcuts hook.
 *
 * Mute / deafen bindings are read from voiceStore so the user can rebind
 * them in Settings → Voice. Ctrl+K (quick switcher) stays hardcoded.
 *
 * Mute / deafen detection depends on runtime:
 * - Electron: native global hook (uIOhook) via IPC, so toggles fire even when
 *   the window is unfocused (e.g. in a game). The document path below skips
 *   mute/deafen in Electron to avoid double-firing when focused.
 * - Browser: document-level keydown (only works when the window is focused).
 *
 * Singleton — called once in AppLayout.
 */

import { useEffect } from "react";
import { useUIStore } from "../stores/uiStore";
import { useVoiceStore } from "../stores/voiceStore";
import type { ShortcutBinding } from "../stores/slices/voiceSettingsSlice";
import { isElectron } from "../utils/constants";

type KeyboardShortcutActions = {
  toggleMute: () => void;
  toggleDeafen: () => void;
};

function matchesBinding(e: KeyboardEvent, binding: ShortcutBinding): boolean {
  return (
    e.code === binding.code &&
    e.ctrlKey === binding.ctrl &&
    e.shiftKey === binding.shift &&
    e.altKey === binding.alt
  );
}

export function useKeyboardShortcuts({ toggleMute, toggleDeafen }: KeyboardShortcutActions) {
  const muteShortcut = useVoiceStore((s) => s.muteShortcut);
  const deafenShortcut = useVoiceStore((s) => s.deafenShortcut);

  // ─── Document-level: Ctrl+K always; mute/deafen only in browser ───
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const target = e.target as HTMLElement;
      const isInputFocused =
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable;

      // Ctrl+K — Quick Switcher (works in input too)
      if (e.ctrlKey && !e.shiftKey && !e.altKey && e.code === "KeyK") {
        e.preventDefault();
        useUIStore.getState().toggleQuickSwitcher();
        return;
      }

      // In Electron the native global hook handles mute/deafen.
      if (isElectron()) return;
      if (isInputFocused) return;

      const { muteShortcut: mute, deafenShortcut: deafen } = useVoiceStore.getState();
      if (matchesBinding(e, mute)) {
        e.preventDefault();
        toggleMute();
        return;
      }
      if (matchesBinding(e, deafen)) {
        e.preventDefault();
        toggleDeafen();
        return;
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [toggleMute, toggleDeafen]);

  // ─── Electron: native global mute/deafen via uIOhook IPC ───
  useEffect(() => {
    if (!isElectron()) return;
    const api = window.electronAPI!;

    api.removeMuteListeners();
    api.removeDeafenListeners();
    api.onMuteGlobalToggle(() => toggleMute());
    api.onDeafenGlobalToggle(() => toggleDeafen());

    api.registerMuteShortcut(muteShortcut);
    api.registerDeafenShortcut(deafenShortcut);

    return () => {
      api.unregisterMuteShortcut();
      api.unregisterDeafenShortcut();
      api.removeMuteListeners();
      api.removeDeafenListeners();
    };
  }, [toggleMute, toggleDeafen, muteShortcut, deafenShortcut]);
}

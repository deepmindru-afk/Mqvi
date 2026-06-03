/**
 * useKeyboardShortcuts — Global keyboard shortcuts hook.
 *
 * Mute / deafen bindings are read from voiceStore so the user can rebind
 * them in Settings → Voice. Ctrl+K (quick switcher) stays hardcoded.
 *
 * Singleton — called once in AppLayout. Document-level capture.
 */

import { useEffect } from "react";
import { useUIStore } from "../stores/uiStore";
import { useVoiceStore } from "../stores/voiceStore";
import type { ShortcutBinding } from "../stores/slices/voiceSettingsSlice";

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

      if (isInputFocused) return;

      const { muteShortcut, deafenShortcut } = useVoiceStore.getState();
      if (matchesBinding(e, muteShortcut)) {
        e.preventDefault();
        toggleMute();
        return;
      }
      if (matchesBinding(e, deafenShortcut)) {
        e.preventDefault();
        toggleDeafen();
        return;
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [toggleMute, toggleDeafen]);
}

import { describe, it, expect, vi } from "vitest";
import { isMouseBinding, DOM_BUTTON_TO_MOUSE_TOKEN } from "./voiceSettingsSlice";

vi.mock("../preferencesStore", () => ({
  usePreferencesStore: { getState: () => ({ set: vi.fn() }) },
}));

describe("isMouseBinding", () => {
  it("should be true for mouse tokens", () => {
    expect(isMouseBinding("Mouse3")).toBe(true);
    expect(isMouseBinding("Mouse4")).toBe(true);
    expect(isMouseBinding("Mouse5")).toBe(true);
  });

  it("should be false for keyboard codes", () => {
    expect(isMouseBinding("KeyV")).toBe(false);
    expect(isMouseBinding("CapsLock")).toBe(false);
    expect(isMouseBinding("Space")).toBe(false);
  });
});

describe("DOM_BUTTON_TO_MOUSE_TOKEN", () => {
  it("should map middle/back/forward and exclude left/right", () => {
    expect(DOM_BUTTON_TO_MOUSE_TOKEN[1]).toBe("Mouse3"); // middle
    expect(DOM_BUTTON_TO_MOUSE_TOKEN[3]).toBe("Mouse4"); // back
    expect(DOM_BUTTON_TO_MOUSE_TOKEN[4]).toBe("Mouse5"); // forward
    expect(DOM_BUTTON_TO_MOUSE_TOKEN[0]).toBeUndefined(); // left
    expect(DOM_BUTTON_TO_MOUSE_TOKEN[2]).toBeUndefined(); // right
  });
});

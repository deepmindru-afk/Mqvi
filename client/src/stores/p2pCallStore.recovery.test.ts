import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const { fetchIceServers, fetchIceServersForRecovery } = vi.hoisted(() => ({
  fetchIceServers: vi.fn(),
  fetchIceServersForRecovery: vi.fn(),
}));
vi.mock("../api/calls", () => ({ fetchIceServers, fetchIceServersForRecovery }));
vi.mock("../i18n", () => ({ default: { t: (k: string) => k } }));
vi.mock("./toastStore", () => ({ useToastStore: { getState: () => ({ addToast: vi.fn() }) } }));

import { createPeerConnection } from "./p2pCallStore";
import type { P2PCall } from "../types";

const REFRESH_SERVERS = [{ urls: "stun:refreshed" }];

// Minimal fake RTCPeerConnection — only what the recovery path touches.
function fakePC() {
  return {
    connectionState: "new" as RTCPeerConnectionState,
    signalingState: "stable" as RTCSignalingState,
    onicecandidate: null as ((e: unknown) => void) | null,
    ontrack: null as ((e: unknown) => void) | null,
    onconnectionstatechange: null as (() => void) | null,
    onnegotiationneeded: null as (() => void) | null,
    restartIce: vi.fn(),
    setConfiguration: vi.fn(),
    getConfiguration: () => ({}),
    close: vi.fn(),
    getSenders: () => [],
    getReceivers: () => [],
  };
}

let pc: ReturnType<typeof fakePC>;
const originalRTCPeerConnection = globalThis.RTCPeerConnection;

beforeEach(() => {
  vi.useFakeTimers();
  fetchIceServers.mockReset().mockResolvedValue(REFRESH_SERVERS);
  fetchIceServersForRecovery.mockReset().mockResolvedValue(REFRESH_SERVERS);
  pc = fakePC();
  // A plain function (not an arrow) so `new RTCPeerConnection()` is valid; returning
  // an object makes the constructor yield our fake.
  globalThis.RTCPeerConnection = function FakeRTCPeerConnection() {
    return pc;
  } as unknown as typeof RTCPeerConnection;
});

afterEach(() => {
  vi.useRealTimers();
  globalThis.RTCPeerConnection = originalRTCPeerConnection;
});

function makeCall(): P2PCall {
  return {
    id: "c1",
    caller_id: "A",
    caller_username: "a",
    caller_display_name: null,
    caller_avatar: null,
    receiver_id: "B",
    receiver_username: "b",
    receiver_display_name: null,
    receiver_avatar: null,
    call_type: "voice",
    status: "active",
    created_at: "",
  };
}

// Builds a PC via the real factory with a fake store, and marks it current.
function harness(isCaller: boolean) {
  const call = makeCall();
  const store: Record<string, unknown> = {
    peerConnection: null,
    activeCall: call,
    endCall: vi.fn(),
    _triggerIceRestart: null,
  };
  const set = (p: unknown) =>
    Object.assign(store, typeof p === "function" ? (p as (s: unknown) => object)(store) : p);
  const get = () => store as never;
  const sendWS = vi.fn();

  const created = createPeerConnection(call, [], isCaller, sendWS, set as never, get);
  store.peerConnection = created; // simulate set({ peerConnection: pc })
  return { store, sendWS, pc: created as unknown as ReturnType<typeof fakePC> };
}

function fail() {
  pc.connectionState = "failed";
  pc.onconnectionstatechange?.();
}

describe("ICE-restart recovery", () => {
  it("caller refreshes credentials and restarts ICE on failure", async () => {
    const { pc: conn } = harness(true);
    fail();
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.setConfiguration).toHaveBeenCalledWith({ iceServers: REFRESH_SERVERS });
    expect(conn.restartIce).toHaveBeenCalledTimes(1);
  });

  it("receiver requests a restart instead of calling restartIce", async () => {
    const { pc: conn, sendWS } = harness(false);
    fail();
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.restartIce).not.toHaveBeenCalled();
    expect(sendWS).toHaveBeenCalledWith(
      "p2p_signal",
      expect.objectContaining({ type: "ice-restart", call_id: "c1" }),
    );
  });

  it("retries up to the cap, then ends the call", async () => {
    const { pc: conn, store } = harness(true);
    fail();
    await vi.advanceTimersByTimeAsync(0); // attempt 1
    expect(conn.restartIce).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(7000); // attempt 2
    expect(conn.restartIce).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(7000); // cap reached
    expect(store.endCall).toHaveBeenCalledTimes(1);
  });

  it("stops retrying once reconnected", async () => {
    const { pc: conn, store } = harness(true);
    fail();
    await vi.advanceTimersByTimeAsync(0); // attempt 1
    conn.connectionState = "connected";
    conn.onconnectionstatechange?.();
    await vi.advanceTimersByTimeAsync(7000);
    expect(conn.restartIce).toHaveBeenCalledTimes(1); // no further attempts
    expect(store.endCall).not.toHaveBeenCalled();
  });

  it("an incoming ice-restart request drives the caller's recovery", async () => {
    const { store, pc: conn } = harness(true);
    expect(typeof store._triggerIceRestart).toBe("function");
    (store._triggerIceRestart as () => void)();
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.restartIce).toHaveBeenCalledTimes(1);
  });

  it("receiver exposes no recovery trigger", () => {
    const { store } = harness(false);
    expect(store._triggerIceRestart).toBeNull();
  });

  it("a superseded PC cannot trigger recovery", async () => {
    const { pc: conn, store } = harness(true);
    store.peerConnection = {}; // a newer PC replaced us
    fail();
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.restartIce).not.toHaveBeenCalled();
  });

  it("keeps the existing config when the recovery fetch fails (no STUN downgrade)", async () => {
    fetchIceServersForRecovery.mockResolvedValue(null);
    const { pc: conn } = harness(true);
    fail();
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.setConfiguration).not.toHaveBeenCalled(); // didn't strip TURN
    expect(conn.restartIce).toHaveBeenCalledTimes(1); // still restarts with existing config
  });

  it("does not restart if the connection recovers during the credential fetch", async () => {
    let resolveFetch: (v: RTCIceServer[] | null) => void = () => {};
    fetchIceServersForRecovery.mockReturnValue(
      new Promise<RTCIceServer[] | null>((r) => {
        resolveFetch = r;
      }),
    );
    const { pc: conn } = harness(true);
    fail(); // starts recovery, suspends on the pending fetch
    conn.connectionState = "connected"; // recovers on its own meanwhile
    conn.onconnectionstatechange?.();
    resolveFetch(REFRESH_SERVERS);
    await vi.advanceTimersByTimeAsync(0);
    expect(conn.restartIce).not.toHaveBeenCalled();
  });
});

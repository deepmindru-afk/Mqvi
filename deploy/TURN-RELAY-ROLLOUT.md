# P2P TURN Relay — Rollout & Test Checklist (Phases 25–28)

What shipped: 1-on-1 P2P calls now fall back to a **coturn relay** when direct
(STUN) connectivity fails, and **recover mid-call** via ICE restart. Server voice
channels (LiveKit) are deliberately **untouched**.

Pieces:
- **Phase 25 (backend):** `GET /api/calls/ice-servers` (active-call gated,
  rate-limited, `no-store`), HMAC TURN credentials, `TURN_SECRET`/`TURN_URLS` env.
- **Phase 26 (frontend):** fetch ICE servers per call, STUN-only fallback.
- **Phase 27 (frontend):** ICE-restart mid-call recovery.
- **Phase 28 (deploy):** `coturn-setup.sh` / `.ps1`, `install.sh` block.

---

## Part 0 — Commit & merge (do first)

- [ ] `git add deploy/coturn-setup.sh deploy/coturn-setup.ps1 deploy/install.sh`
- [ ] Commit, push the branch, open PR, **merge to `main`**
      (why: `install.sh` downloads `coturn-setup.sh` from `main`).
- [ ] The frontend (Phase 26/27) lives in the client. For testing, run it where
      you can inspect WebRTC — easiest is the **web/dev build in Chrome**
      (`npm run dev`, two windows/profiles) since `chrome://webrtc-internals` is
      available there. Ship a new desktop release for end users after prod passes.

---

## Part 1 — Set up the TEST server  (`root@46.225.191.119`)

Order matters: **coturn reads `TURN_SECRET` from `.env`**, so set it first.

1. [ ] SSH in and set the shared secret (only if not already present):
   ```bash
   ssh root@46.225.191.119
   cd ~/mqvi
   grep -q '^TURN_SECRET=' .env || echo "TURN_SECRET=$(openssl rand -hex 32)" >> .env
   grep TURN_SECRET .env        # confirm it's there
   ```
2. [ ] From your machine, run the **one-time** coturn setup:
   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\coturn-setup.ps1
   ```
   (installs coturn, writes `/etc/turnserver.conf` from the `.env` secret, sets
   `TURN_URLS`, opens the host firewall, starts coturn)
3. [ ] Open the **Hetzner cloud firewall** for the test server:
   `3478/udp`, `3478/tcp`, `49152-49999/udp`
4. [ ] Deploy the new backend (it then loads `TURN_SECRET` + `TURN_URLS`):
   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\redeploy.ps1
   ```

> coturn is **independent of redeploys** — do NOT re-run `coturn-setup` on every
> deploy. Only once per host (or if the public IP / secret changes).

---

## Part 2 — Test checklist (run on TEST before prod)

### A. coturn health (on the server)
- [ ] `systemctl status coturn` → **active (running)**
- [ ] `journalctl -u coturn -n 40` → no errors; shows listeners on `3478`
- [ ] `ss -lun | grep 3478` and `ss -ltn | grep 3478` → UDP + TCP listening
- [ ] `ls -l /etc/turnserver.conf` → `-rw-r-----` (640), group `turnserver`/`coturn`
- [ ] Survives reboot: `reboot`, then `systemctl is-active coturn` → active

### B. Backend picked up the config
- [ ] `grep -E 'TURN_SECRET|TURN_URLS' ~/mqvi/.env` → both present; `TURN_URLS`
      uses the public IP
- [ ] Admin panel → Logs → INFO entry **"TURN relay configured: 1 server(s)…"**
      (NOT "TURN relay not configured" — that means STUN-only)

### C. coturn auth + relay — standalone (optional but fast)
On a Linux box with the same secret (flags vary by version — see `turnutils_uclient -h`):
- [ ] `turnutils_uclient -y -u tester -W '<TURN_SECRET>' <PUBLIC_IP>` → succeeds
      - auth fails → `TURN_SECRET` mismatch between `.env` and `turnserver.conf`
      - times out → firewall (host or Hetzner) blocking 3478 / relay ports

### D. P2P call — normal (direct) path
Two clients, friends, on normal networks:
- [ ] Voice call connects, two-way audio
- [ ] Video call connects, two-way video
- [ ] `chrome://webrtc-internals` → selected candidate pair is **host/srflx (NOT
      relay)** — confirms direct calls don't touch server bandwidth

### E. P2P call — RELAY path (the key TURN test)
Force a relay: put **one** client behind symmetric NAT (phone hotspot) or block
its direct UDP:
- [ ] Call still connects
- [ ] `chrome://webrtc-internals` → selected candidate pair type is **`relay`**
      (address = your coturn public IP)
- [ ] `journalctl -u coturn -f` during the call → allocation/session logs appear

### F. Mid-call recovery (Phase 27)
During an active call:
- [ ] Briefly drop one client's network (toggle WiFi / switch to hotspot) → the
      call **recovers within ~15s** instead of ending
- [ ] Devtools console shows `ICE restart attempt 1/2`
- [ ] If the network stays down past the cap → call ends **cleanly** (no stuck UI)

### G. P2P hardening behaviors
- [ ] **Busy**: A calls C; while ringing, B calls C → **B gets "busy"** (C does
      not get a second incoming call)
- [ ] **Ringing timeout**: an unanswered call auto-ends after ~60s
- [ ] Decline / hang up tears down both sides; no ghost call/tab

### H. Regression — server VOICE CHANNELS unaffected (critical)
- [ ] Join a server voice channel → audio works
- [ ] Camera + screen share in the voice channel work
- [ ] Switching servers / channels still works
      (TURN changes must not touch LiveKit voice — verify, don't assume)

### I. Graceful degrade
- [ ] `systemctl stop coturn`, make a P2P call on a normal network → **still
      connects** (STUN-only). Then `systemctl start coturn`.

---

## Part 3 — Promote to PROD  (`root@46.225.124.90`) — only after TEST passes

1. [ ] `ssh root@46.225.124.90`, set `TURN_SECRET` in `~/mqvi/.env` (as in Part 1.1)
2. [ ] `powershell -ExecutionPolicy Bypass -File deploy\coturn-setup.ps1 -Server root@46.225.124.90`
3. [ ] Open the **prod** Hetzner cloud firewall: `3478/udp`, `3478/tcp`, `49152-49999/udp`
4. [ ] `powershell -ExecutionPolicy Bypass -File deploy\redeploy-prod.ps1`
5. [ ] Re-run checklist **A–C** + a quick **D, H** on prod
6. [ ] Release the new desktop app so end users get the Phase 26/27 frontend

---

## Tuning / notes
- `TURN_SECRET` must be **identical** in `~/mqvi/.env` and `/etc/turnserver.conf`
  (the setup script keeps them in sync by reading the `.env`).
- Credential TTL is 24h (`TURN_CREDENTIAL_TTL_SECONDS`); relay caps are
  `user-quota=12`, `total-quota=1200`, `max-bps=500000` (~4 Mbit/s per session).
  Edit `/etc/turnserver.conf` then `systemctl restart coturn` to change them.
- coturn relay ports (`49152-49999`) sit **below** LiveKit's (`50000+`) on purpose
  — never let them overlap on a shared host.

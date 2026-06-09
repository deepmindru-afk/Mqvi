# Self-hosting

mqvi can run on your own infrastructure. There are **two levels** — pick based on how independent you want to be.

## Two paths

**1. Voice server only** *(easiest)*
Keep using your mqvi.net account — friends, DMs, and servers all work as normal — but route your **voice and video** through your own server. Your calls never touch our infrastructure. One command to set up.

**2. Full server** *(fully independent)*
Run the **entire platform** yourself — accounts, messages, files, and voice. Completely separate from mqvi.net; you control everything. One command to set up.

| | Voice only | Full server |
| --- | --- | --- |
| Account | mqvi.net | your own |
| Setup | LiveKit script | install script |
| Specs | 1 GB RAM, 1 core | 4 GB RAM, 2 vCPU |
| Cost | ~$3–5/mo VPS | a small VPS |

## What you'll need

- A **Linux server** (Ubuntu 22.04+ / Debian 12+ recommended). Hetzner, DigitalOcean, or Contabo all work well.
- For the full server, a **domain is optional** — without one, the installer uses a free `sslip.io` hostname so HTTPS still works. (Browsers block microphone, camera, and screen-share over plain HTTP, so HTTPS matters.)

The next two pages walk through each path, step by step.

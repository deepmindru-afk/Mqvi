#!/bin/bash
# mqvi — Start both LiveKit SFU and mqvi backend
#
# Usage: chmod +x start.sh mqvi-server && ./start.sh
#
# Ctrl+C gracefully stops both processes.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# ─── Check .env ───
if [ ! -f .env ]; then
    echo "ERROR: .env file not found!"
    echo "Copy the example: cp .env.example .env"
    echo "Then set JWT_SECRET and LIVEKIT_API_SECRET."
    exit 1
fi

# ─── Check / download LiveKit binary ───
if [ ! -f ./livekit-server ]; then
    echo "LiveKit server not found. Downloading..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) LK_ARCH="amd64" ;;
        aarch64|arm64) LK_ARCH="arm64" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    LK_VERSION="v1.9.12"
    LK_URL="https://github.com/livekit/livekit/releases/download/${LK_VERSION}/livekit_${LK_VERSION#v}_linux_${LK_ARCH}.tar.gz"
    echo "Downloading: $LK_URL"
    curl -fsSL "$LK_URL" | tar xz livekit-server
    chmod +x livekit-server
    echo "LiveKit server downloaded."
fi

# ─── Create data directories ───
mkdir -p data/uploads

env_value() {
    # $1 = key, $2 = default
    local key="$1"
    local default="$2"
    local line value
    line="$(grep -E "^${key}=" .env 2>/dev/null | tail -n 1 || true)"
    if [ -z "$line" ]; then
        printf '%s' "$default"
        return
    fi
    value="${line#*=}"
    value="${value%\"}"
    value="${value#\"}"
    printf '%s' "$value"
}

wait_for_tcp() {
    # $1 = host, $2 = port, $3 = attempts
    local host="$1"
    local port="$2"
    local attempts="$3"
    local i
    for i in $(seq 1 "$attempts"); do
        if (echo >"/dev/tcp/${host}/${port}") >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    return 1
}

ensure_clamav_ready() {
    local enabled addr host port
    enabled="$(env_value MQVI_ANTIVIRUS_ENABLED true)"
    case "$(printf '%s' "$enabled" | tr '[:upper:]' '[:lower:]')" in
        false|0|no|off)
            echo "[start] Antivirus scanning disabled."
            return
            ;;
    esac

    addr="$(env_value MQVI_CLAMAV_ADDR 127.0.0.1:3310)"
    host="${addr%:*}"
    port="${addr##*:}"
    if [ -z "$host" ] || [ -z "$port" ] || [ "$host" = "$port" ]; then
        echo "[start] WARNING: invalid MQVI_CLAMAV_ADDR=${addr}; uploads will follow antivirus unavailable policy."
        return
    fi

    if command -v systemctl >/dev/null 2>&1; then
        systemctl start clamav-daemon >/dev/null 2>&1 || true
        systemctl start clamd@scan >/dev/null 2>&1 || true
    fi

    echo "[start] Waiting for ClamAV at ${addr}..."
    if wait_for_tcp "$host" "$port" 10; then
        echo "[start] ClamAV is reachable."
    else
        echo "[start] WARNING: ClamAV is not reachable at ${addr}; uploads will follow antivirus unavailable policy."
    fi
}

echo "========================================="
echo "  Starting mqvi server..."
echo "========================================="
echo ""

# ─── Start LiveKit in the background ───
echo "[start] Starting LiveKit SFU (port 7880)..."
./livekit-server --config livekit.yaml &
LIVEKIT_PID=$!

# ─── Cleanup trap — stop both on Ctrl+C or SIGTERM ───
cleanup() {
    echo ""
    echo "[start] Stopping servers..."
    kill $LIVEKIT_PID 2>/dev/null || true
    kill $MQVI_PID 2>/dev/null || true
    wait $LIVEKIT_PID 2>/dev/null || true
    wait $MQVI_PID 2>/dev/null || true
    echo "[start] Clean shutdown complete."
    exit 0
}
trap cleanup SIGINT SIGTERM

# Wait briefly for LiveKit to start
sleep 1

ensure_clamav_ready

# ─── Start mqvi backend ───
echo "[start] Starting mqvi backend (port 9090)..."
./mqvi-server &
MQVI_PID=$!

echo ""
echo "========================================="
echo "  mqvi is running!"
echo "  Web UI:  http://$(hostname -I | awk '{print $1}'):9090"
echo "  LiveKit: ws://localhost:7880"
echo "  Stop with: Ctrl+C"
echo "========================================="
echo ""

# Wait for either process — if one dies, stop both
wait -n $LIVEKIT_PID $MQVI_PID 2>/dev/null || true
cleanup

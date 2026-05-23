#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
LOCK_FILE="$ROOT_DIR/data/.data-update.lock"
TMP_DIR="$ROOT_DIR/data/.tmp-update"
SOCKS_SOCKET="$TMP_DIR/relay-socks.socket"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing .env file: $ENV_FILE" >&2
  exit 1
fi

mkdir -p "$ROOT_DIR/data"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "data update already running" >&2
  exit 0
fi

set -a
source "$ENV_FILE"
set +a

GAR_OVERLAY_REGIONS="${GAR_OVERLAY_REGIONS:-77,78}"
KLADR_SOURCE_PATH="${KLADR_SOURCE_PATH:-/app/data/BASE.7z}"
GAR_DATA_PATH="${GAR_DATA_PATH:-/app/data/gar_overlay.json}"
DOWNLOAD_RELAY_HOST="${DOWNLOAD_RELAY_HOST:-}"
DOWNLOAD_RELAY_SSH_KEY="${DOWNLOAD_RELAY_SSH_KEY:-/root/.ssh/id_rsa_georgri_github}"
RELAY_SOCKS_PORT="${RELAY_SOCKS_PORT:-1080}"
APP_IMAGE="${APP_IMAGE:-sledopyt-addresses_sledopyt-addresses}"

cleanup_tunnel() {
  if [[ -n "$DOWNLOAD_RELAY_HOST" ]]; then
    ssh -S "$SOCKS_SOCKET" -O exit "$DOWNLOAD_RELAY_HOST" >/dev/null 2>&1 || true
  fi
}
trap cleanup_tunnel EXIT

if [[ -n "$DOWNLOAD_RELAY_HOST" ]]; then
  if [[ ! -f "$DOWNLOAD_RELAY_SSH_KEY" ]]; then
    echo "relay ssh key not found: $DOWNLOAD_RELAY_SSH_KEY" >&2
    exit 1
  fi
  rm -f "$SOCKS_SOCKET"
  ssh -i "$DOWNLOAD_RELAY_SSH_KEY" \
    -o ExitOnForwardFailure=yes \
    -o StrictHostKeyChecking=accept-new \
    -o UserKnownHostsFile=/root/.ssh/known_hosts \
    -M -S "$SOCKS_SOCKET" -fnNT \
    -D "127.0.0.1:${RELAY_SOCKS_PORT}" \
    "$DOWNLOAD_RELAY_HOST"
  export HTTPS_PROXY="socks5://127.0.0.1:${RELAY_SOCKS_PORT}"
  export HTTP_PROXY="$HTTPS_PROXY"
  export ALL_PROXY="$HTTPS_PROXY"
  export NO_PROXY="localhost,127.0.0.1"
  echo "Using relay host ${DOWNLOAD_RELAY_HOST} via SOCKS on 127.0.0.1:${RELAY_SOCKS_PORT}"
fi

if [[ "$KLADR_SOURCE_PATH" != /app/data/* ]]; then
  echo "KLADR_SOURCE_PATH must point to /app/data inside container, got: $KLADR_SOURCE_PATH" >&2
  exit 1
fi
if [[ "$GAR_DATA_PATH" != /app/data/* ]]; then
  echo "GAR_DATA_PATH must point to /app/data inside container, got: $GAR_DATA_PATH" >&2
  exit 1
fi

KLADR_HOST_PATH="$ROOT_DIR/data/${KLADR_SOURCE_PATH#/app/data/}"
GAR_HOST_PATH="$ROOT_DIR/data/${GAR_DATA_PATH#/app/data/}"
KLADR_NEXT="$KLADR_HOST_PATH.next"
GAR_NEXT="$GAR_HOST_PATH.next"

mkdir -p "$(dirname "$KLADR_HOST_PATH")" "$(dirname "$GAR_HOST_PATH")" "$TMP_DIR"

TARGET_DATE=""
KLADR_URL=""
GAR_URL=""
for day_shift in $(seq 1 14); do
  ds="$(date -u -d "-${day_shift} day" +%Y.%m.%d)"
  iso="$(date -u -d "-${day_shift} day" +%F)"
  kladr_candidate="https://fias-file.nalog.ru/downloads/${ds}/base.7z"
  gar_candidate="https://fias-file.nalog.ru/downloads/${ds}/gar_xml.zip"
  if curl -fsSI --connect-timeout 12 --max-time 20 "$kladr_candidate" >/dev/null &&
    curl -fsSI --connect-timeout 12 --max-time 20 "$gar_candidate" >/dev/null; then
    TARGET_DATE="$iso"
    KLADR_URL="$kladr_candidate"
    GAR_URL="$gar_candidate"
    break
  fi
done
if [[ -z "$TARGET_DATE" || -z "$KLADR_URL" || -z "$GAR_URL" ]]; then
  echo "no suitable date found on fias-file.nalog.ru for last 14 days" >&2
  exit 1
fi

echo "Using data version date: $TARGET_DATE"
echo "Downloading KLADR BASE.7z..."
curl -fL --retry 3 --retry-delay 5 "$KLADR_URL" -o "$KLADR_NEXT"

echo "Building GAR overlay for regions: $GAR_OVERLAY_REGIONS"
if ! /usr/bin/docker image inspect "$APP_IMAGE" >/dev/null 2>&1; then
  /usr/bin/docker-compose -f "$ROOT_DIR/docker-compose.yml" build sledopyt-addresses
fi
/usr/bin/docker run --rm \
  --network host \
  --entrypoint /usr/local/bin/gar-fetch \
  -e GOMEMLIMIT=1GiB \
  -e GOGC=50 \
  -e HTTPS_PROXY="${HTTPS_PROXY:-}" \
  -e HTTP_PROXY="${HTTP_PROXY:-}" \
  -e ALL_PROXY="${ALL_PROXY:-}" \
  -e NO_PROXY="${NO_PROXY:-}" \
  -v "$ROOT_DIR/data:/app/data" \
  "$APP_IMAGE" \
  -zip-url "$GAR_URL" \
  -regions "$GAR_OVERLAY_REGIONS" \
  -out "/app/data/${GAR_DATA_PATH#/app/data/}.next"

test -s "$KLADR_NEXT"
test -s "$GAR_NEXT"

mv "$KLADR_NEXT" "$KLADR_HOST_PATH"
mv "$GAR_NEXT" "$GAR_HOST_PATH"

echo "Restarting bot container to pick up fresh data..."
/usr/bin/docker-compose -f "$ROOT_DIR/docker-compose.yml" up -d --no-build sledopyt-addresses
/usr/bin/docker-compose -f "$ROOT_DIR/docker-compose.yml" logs --tail=30 sledopyt-addresses

echo "KLADR + GAR update completed."

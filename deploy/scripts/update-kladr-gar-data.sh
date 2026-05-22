#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
LOCK_FILE="$ROOT_DIR/data/.data-update.lock"
TMP_DIR="$ROOT_DIR/data/.tmp-update"

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

read -r TARGET_DATE KLADR_URL GAR_URL < <(
  python3 - <<'PY'
import datetime as dt
import urllib.request

for day_shift in range(1, 15):
    d = dt.date.today() - dt.timedelta(days=day_shift)
    ds = d.strftime("%Y.%m.%d")
    kladr = f"https://fias-file.nalog.ru/downloads/{ds}/base.7z"
    gar = f"https://fias-file.nalog.ru/downloads/{ds}/gar_xml.zip"
    ok = True
    for url in (kladr, gar):
        req = urllib.request.Request(url, method="HEAD")
        try:
            with urllib.request.urlopen(req, timeout=12) as resp:
                if resp.status // 100 != 2:
                    ok = False
                    break
        except Exception:
            ok = False
            break
    if ok:
        print(d.isoformat(), kladr, gar)
        raise SystemExit(0)

raise SystemExit("no suitable date found on fias-file.nalog.ru for last 14 days")
PY
)

echo "Using data version date: $TARGET_DATE"
echo "Downloading KLADR BASE.7z..."
curl -fL --retry 3 --retry-delay 5 "$KLADR_URL" -o "$KLADR_NEXT"

echo "Building GAR overlay for regions: $GAR_OVERLAY_REGIONS"
/usr/bin/docker-compose -f "$ROOT_DIR/docker-compose.yml" run --rm --no-deps \
  -e GOMEMLIMIT=1GiB \
  -e GOGC=50 \
  --entrypoint /usr/local/bin/gar77-fetch \
  sledopyt-addresses \
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

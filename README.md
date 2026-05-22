# sledopyt-addresses

Telegram bot for city orienteering game "Sledopyt".  
It finds `street + house` variants that satisfy a user formula (for example: `1 + 3*x2 - 4*x5 + "б"`), using official KLADR data (`BASE.7z` from nalog/FIAS updates) and optional GAR overlay for selected regions.

## Features

- Long polling Telegram bot (no webhook needed).
- Fuzzy city search by name (`/cityname <query>`, up to 50 variants).
- City selection from loaded KLADR dataset (`/city <code>`).
- Per-user city persistence in local JSON (`user_id -> city_code`).
- Formula parsing, normalization, validation.
- Search by formula over chosen city + all its descendant KLADR codes.
- Pagination for matches (`/more`, 50 results per page).
- Optional GAR overlay: municipal/admin regions as selectable "cities" with their streets/houses.
- Dockerized deployment.

## Formula syntax

Supported:

- integers (`1`, `-4`)
- variables `x1`, `x2`, ... (`xN` = number in Russian alphabet of N-th letter in street name)
- `+`, `-`
- multiplication `k*xN` (example: `3*x2`)
- optional suffix letter in quotes: `+ "б"`

Example:

```text
1 + 3*x2 - 4*x5 + "б"
```

## KLADR source and parsing

Official source format:

- archive: `BASE.7z`
- tables used:
  - `KLADR.DBF` (settlements metadata/name)
  - `STREET.DBF` (streets)
  - `DOMA.DBF` (houses)

The service accepts `KLADR_SOURCE_PATH` as:

- path to `BASE.7z`, or
- path to already extracted directory containing these DBF files.

## GAR overlay for selected regions (optional)

To add administrative/municipal regions with streets/houses, generate overlay JSON from official `gar_xml.zip` without downloading full archive:

```bash
go run ./cmd/gar77-fetch -regions 77,78 -out ./data/gar_overlay.json
```

The utility:

- reads ZIP index over HTTP range requests,
- downloads only required per-region files (`AS_ADDR_OBJ`, `AS_HOUSES`, `AS_ADM_HIERARCHY`, `AS_MUN_HIERARCHY`),
- builds compact JSON with regions -> streets -> houses.

Then set `GAR_DATA_PATH` to this JSON file.

## In-memory structures and complexity

To keep search fast and predictable:

- `AddressIndex.Cities map[string]*City`
- `City.Streets []*Street`
- `Street.Houses map[string]struct{}`
- `Street.LetterValues []int` (precomputed Russian letter indices)

Search steps for chosen city:

1. Iterate streets once.
2. Evaluate formula using precomputed `LetterValues`.
3. Check existence of produced house number in `Houses` hash set.

Complexity:

- `O(S_city)` time where `S_city` is streets count in city.
- House lookup is `O(1)` average due to hash set.
- This is not worse than linear in number of city houses and usually significantly faster.

Memory efficiency choices:

- Keep only required fields from DBF rows.
- Store house numbers as normalized lowercase strings.
- Reuse city/street maps by code prefixes.

## Configuration

Environment variables:

- `TELEGRAM_BOT_TOKEN` - required.
- `KLADR_SOURCE_PATH` - required (`/app/data/BASE.7z` in Docker example).
- `GAR_DATA_PATH` - optional path to generated GAR overlay JSON.
- `USER_STATE_PATH` - optional (`data/user_state.json` default).

Example:

```bash
cp .env.example .env
# edit .env
```

## Local run

```bash
go mod tidy
TELEGRAM_BOT_TOKEN=... \
KLADR_SOURCE_PATH=/absolute/path/to/BASE.7z \
GAR_DATA_PATH=./data/gar_overlay.json \
USER_STATE_PATH=./data/user_state.json \
go run ./cmd/sledopyt-addresses
```

## Docker run

```bash
cp .env.example .env
# set real token and paths in .env
docker compose up --build -d
docker compose logs -f
```

## VPS deployment (systemd + Docker Compose)

On server:

```bash
mkdir -p ~/sledopyt-addresses/data
cd ~/sledopyt-addresses
# copy project files here
cp .env.example .env
# set TELEGRAM_BOT_TOKEN and KLADR_SOURCE_PATH in .env
```

Install and start service:

```bash
sudo cp deploy/systemd/sledopyt-addresses.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sledopyt-addresses
sudo systemctl status sledopyt-addresses
```

## Testing for large cities (Moscow / Moscow region)

Recommended check on VPS:

1. Load full `BASE.7z`.
2. In Telegram:
   - `/cityname москва`
   - `/city <moscow_code>`
   - send a formula with common house result.
3. Observe memory and CPU:
   - `docker stats`
   - `journalctl -u sledopyt-addresses -f`

With 8 GB RAM this design is feasible if only required tables are loaded; monitor startup peak during extraction/loading.

## Notes

- KLADR house fields may contain ranges/lists; parser normalizes and expands short numeric ranges.
- Bot currently uses plain text command flow for reliability in long polling mode.

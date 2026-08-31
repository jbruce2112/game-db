# game-db

Self-hosted catalog for a **physical** video game collection. A Go server is the source of truth and hosts the web UI. The iOS app works fully offline and syncs when the server is reachable.

Game metadata and poster art come from [IGDB](https://www.igdb.com/). Optional platform-specific physical box fronts come from [TheGamesDB](https://thegamesdb.net/).

## Quick start (Docker)

```bash
cp .env.example .env
# set APP_PASSWORD
# set IGDB_CLIENT_ID and IGDB_CLIENT_SECRET (https://api-docs.igdb.com/#getting-started)
# optional: EBAY_CLIENT_ID + EBAY_CLIENT_SECRET for asking prices (https://developer.ebay.com)
# optional: THEGAMESDB_API_KEY for physical box fronts (https://thegamesdb.net/api.php)
docker compose up --build
```

Open http://localhost:8080 and sign in with `APP_PASSWORD`. To publish a different host port, set `HTTP_PORT` in `.env` (for example `HTTP_PORT=8078`) and recreate the container. Compose still uses port 8080 *inside* the container; `HTTP_ADDR` in `.env` is ignored by Compose.

### Other machines on the LAN

This app is **not** on port 80. Other Docker apps on the same IP are often reverse-proxied at `http://192.168.1.50/` or HTTPS. Game-db is published as:

`http://192.168.1.50:8080` (or `http://192.168.1.50:8078` if `HTTP_PORT=8078`).

That host port is required. In the iOS app, Settings must be that full URL (no trailing path).

On the server, confirm the API answers locally first (use your `HTTP_PORT`):

```bash
curl -sS http://127.0.0.1:8080/health
# expect: {"status":"ok"}

ss -tlnp | grep -E '8080|8078'
# want: 0.0.0.0:<HTTP_PORT>  (not only 127.0.0.1)
```

If the port is only on `127.0.0.1`, Docker is publishing localhost-only (common with rootless Docker). The compose file binds `0.0.0.0:8080:8080`. Recreate:

```bash
docker compose down
docker compose up --build
```

If it still does not load from another device, open the port on the host firewall:

```bash
sudo ufw allow 8080/tcp
# or: sudo firewall-cmd --add-port=8080/tcp --permanent && sudo firewall-cmd --reload
```

mDNS names like `big-mama.local` often fail on iPhone; use the numeric IP.

Data (SQLite + covers) lives in `./data`. Create it first if you want:

```bash
mkdir -p data/covers
```

SQLite error 14 (`unable to open database file: out of memory`) almost always means the process cannot write that folder (permissions), not RAM. The Compose image writes as root so a normal `./data` bind mount works.

## Local development

You need Go 1.23+ and Node 20+.

```bash
# terminal 1 — API
cd server
cp .env.example .env   # edit APP_PASSWORD (and IGDB keys if you want search)
go run ./cmd/gamedb

# terminal 2 — Vite (proxies /v1 to :8080)
cd web
npm install
npm run dev
```

Web UI: http://localhost:5173

```bash
cd server && go test ./...
make test-stress           # concurrent CRUD, multi-device sync, CSV, barcodes
STRESS=1 make test-stress  # 10× volume
```

## Barcode

Scan a box UPC/EAN to add a game.

- **iOS:** camera button on the library, or Add → Scan. The Simulator has no camera; type the digits.
- **Web:** Add game → Barcode. Camera works in Chromium-based browsers that support `BarcodeDetector`; otherwise type the code.

The server looks the code up in a product catalog (upcitemdb, then Open Products Facts, then Go-UPC), caches the result, then searches IGDB with a cleaned title. Grocery catalogs often miss Japanese JAN / Asian SKUs; Go-UPC is the fallback for those. The barcode is stored on the copy and included in CSV export/import. There is no official IGDB barcode field, so a miss still lets you add the title by hand with the code attached.

## iOS

Open `ios/GameDB.xcodeproj` in Xcode (iOS 17+). On Xcode 26, install the matching **iOS platform** from Settings → Components before the Simulator destination appears. Run on the Simulator for local-only use. To sync, set the server URL (e.g. `http://<lan-ip>:8080`) and the same password in Settings.

HTTP on the LAN is allowed (ATS `NSAllowsArbitraryLoads`) because v1 is LAN-first. Prefer Tailscale if you expose the server off-LAN.

A paid Apple Developer account is required to run on a physical iPhone.

The process still prefers real environment variables over the file. Docker Compose continues to use the repo-root `.env`.

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `APP_PASSWORD` | yes | | Shared login password |
| `DATA_DIR` | no | `./data` | SQLite + cover files |
| `HTTP_ADDR` | no | `:8080` | Listen address |
| `COOKIE_SECURE` | no | `0` | Set `1` behind HTTPS |
| `IGDB_CLIENT_ID` | for search | | Twitch/IGDB client id |
| `IGDB_CLIENT_SECRET` | for search | | Twitch/IGDB secret |
| `EBAY_CLIENT_ID` | for values | | eBay App ID (free developer key) |
| `EBAY_CLIENT_SECRET` | for values | | eBay Cert ID |
| `EBAY_MARKETPLACE` | no | `EBAY_US` | eBay marketplace for asking prices |
| `PRICECHARTING_TOKEN` | optional | | Paid PriceCharting token (used if eBay is unset) |
| `THEGAMESDB_API_KEY` | for box art | | Free TheGamesDB key for platform box fronts |

## Layout

See `plan.md` for the v1 design (sync, auth, data model, backlog).

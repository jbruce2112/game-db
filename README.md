# game-db

Self-hosted catalog for a **physical** video game collection. A Go server is the source of truth and hosts the web UI. The iOS app works fully offline and syncs when the server is reachable.

Game metadata and cover art come from [IGDB](https://www.igdb.com/).

## Quick start (Docker)

```bash
cp .env.example .env
# set APP_PASSWORD
# set IGDB_CLIENT_ID and IGDB_CLIENT_SECRET (https://api-docs.igdb.com/#getting-started)
docker compose up --build
```

Open http://localhost:8080 and sign in with `APP_PASSWORD`.

Data (SQLite + covers) lives in `./data`.

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

The server looks the code up in a product catalog (upcitemdb, then Open Products Facts), caches the result, then searches IGDB with a cleaned title. The barcode is stored on the copy and included in CSV export/import. There is no official IGDB barcode field, so a miss still lets you add the title by hand with the code attached.

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

## Layout

See `plan.md` for the v1 design (sync, auth, data model, backlog).

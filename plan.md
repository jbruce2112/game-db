# Physical video game library (game-db) — v1 plan

## Context

Build a self-hosted replacement for the core CLZ Games workflow: catalog a **physical** video game collection, browse it on the web, and manage it on iPhone even when the home server is unreachable. Data lives on hardware you control, not a paid cloud.

This is a greenfield repo (`/Users/john/src/game-db`). v1 is intentionally small. Barcode scan, pricing, wishlists, hardware, and custom fields come later.

## Locked decisions

| Topic | Choice |
|---|---|
| Who | One person, one library, multiple devices |
| Server | Optional. Source of truth **when present**. |
| Web | Server UI only — not a standalone PWA |
| iOS | Fully usable offline / with no server. Sync both ways when the server is reachable. |
| Add-game v1 | Search IGDB by title + platform, snapshot metadata onto the owned copy. Manual add as fallback. |
| iOS client | Native SwiftUI |
| Web client | TypeScript SPA (Vite + React), same origin as the API |
| Server | Go + SQLite, one Docker Compose service on a homelab/NAS |
| Auth | Single shared password. No user accounts in v1. |

## Architecture

```text
┌──────────────────────────────────┐
│  iOS (SwiftUI)                   │
│  local SQLite (GRDB)             │  ← library data; works with no server
│  Keychain: bearer token          │  ← login credential; see below
└────────────────┬─────────────────┘
                 │ HTTPS/HTTP when reachable
                 │ REST + /sync
                 │ Authorization: Bearer <token>
┌────────────────▼─────────────────┐         ┌─────────────┐
│  game-db server (Go)             │────────▶│  IGDB API   │
│  SQLite + cover files            │ cache   │  (Twitch)   │
│  serves API + web dist           │         └─────────────┘
└────────────────▲─────────────────┘
                 │ same origin (cookie session)
┌────────────────┴─────────────────┐
│  Web (React + TS SPA)            │
└──────────────────────────────────┘
```

- **One container** publishes the API and the built SPA. Browser and iOS talk to the same REST API.
- **SQLite** is the database (WAL mode, one volume). A personal library of thousands of games does not need Postgres.
- **IGDB** is only contacted by the **server**. Covers and search results are cached locally so the library keeps working if IGDB is down.
- **iOS without a server:** browse / edit / manually add. IGDB search appears once a server (with IGDB credentials) is configured. No Twitch keys in the app binary.

### What “Keychain token” means on iOS

The phone does **not** send the library password on every request, and it does **not** keep that password in the games database.

Flow:

1. In Settings you enter the server URL + the shared `APP_PASSWORD` once.
2. The app calls `POST /v1/auth/login`. The server checks the password and returns an **opaque bearer token** (a random string the server also stores). That token is the iOS equivalent of the web app’s session cookie.
3. The app writes the token into the **iOS Keychain** — Apple’s encrypted credential store for passwords, tokens, and certificates. It is not `UserDefaults`, not the GRDB/SQLite file, and not a plist in the sandbox. Other apps cannot read it; it survives app restarts.
4. Every later API/sync call sends `Authorization: Bearer <token>`. The password is discarded from memory after login and is **not** persisted.
5. Server URL (not a secret) can live in `UserDefaults`. “Forget server” deletes the Keychain item and the URL, and the app goes back to local-only mode.

Keychain attributes for v1: `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` so the token is available for background sync after first unlock, is **not** included in iCloud/iTunes backups, and does not roam to other devices. One token per install is enough.

If the server rejects the token (password rotated, token revoked), the app clears the Keychain item and prompts for the password again. Offline, the missing/expired token does not matter — GRDB is still the working copy; sync waits until login succeeds.

Remote access later should be Tailscale (or similar), not a forwarded port. v1 is LAN-first; HTTP on the LAN is acceptable.

## Data model

Separate **catalog cache** (IGDB) from **owned copies** (the library). After import, the owned copy is a snapshot the user can edit. IGDB going away must not blank the shelf.

### `library_items` (source of truth for the collection)

One row per physical copy. Duplicates allowed (two copies of the same title).

| Column | Notes |
|---|---|
| `id` | UUID, **client-generated** so offline inserts don't collide |
| `title` | Editable snapshot |
| `platform` | Display name + `igdb_platform_id` |
| `region` | `us` / `eu` / `jp` / `au` / `other` / null |
| `completeness` | `unknown` / `loose` / `cib` / `new` |
| `notes` | Free text |
| `igdb_game_id` | Nullable, for later re-fetch — not unique |
| `cover_id` | Nullable, file on the server volume |
| `barcode` | Nullable UPC/EAN on this copy |
| `created_at` / `updated_at` | UTC RFC3339 |
| `deleted_at` | Tombstone; rows are hidden, not hard-deleted |
| `dirty` | Client-only: needs push |

v1 does **not** include location, purchase price/store/date, condition grade, genres on the row, or quantity>1 as a field (two copies = two rows).

### `igdb_games` / `igdb_platforms` / `covers`

Server-side cache. iOS does not need the full catalog cache; it stores cover images it has downloaded.

Shared SQL lives in `schema/schema.sql` and is the contract for Go (`sqlc`) and a matching GRDB schema on iOS.

## Sync protocol

Single-user, few devices, rare concurrent edits → **state-based last-write-wins**, not CRDTs.

1. Every record has a client-generated UUID and `updated_at`.
2. Deletes are tombstones (`deleted_at` set, `updated_at` bumped).
3. Clients keep a `cursor` (server monotonic `sync_seq`, integer).
4. `POST /v1/sync`:
   - Body: `{ cursor, changes: [LibraryItem...] }` — local dirty rows (including tombstones).
   - Server applies each change: insert if unknown; if known, **LWW on `updated_at`** (tie-break: larger UUID wins).
   - Server assigns/increments `sync_seq` on every accepted write.
   - Response: `{ cursor, changes: [...] }` — all rows with `sync_seq >` the request cursor, minus the ones the client just sent if unchanged.
5. Client upserts the returned rows, clears `dirty` on acknowledged ids, stores the new cursor.
6. Covers are not in the sync payload. After metadata sync, iOS GETs missing `/v1/covers/{id}` when online.

**Known v1 limitation:** LWW is **record-level**. Editing notes on the phone and title on the web at the same time keeps one whole row. Fine for one person; field-level merge is a later upgrade.

**Clocks:** clients write UTC `updated_at` at edit time. Server rejects timestamps unreasonably in the future (e.g. > 5 minutes) and clamps them to now so a wrong phone clock cannot shadow every other device.

**iOS triggers:** app foreground, pull-to-refresh, after a local edit (debounced), and a simple background URLSession if it stays cheap. Failures stay queued via `dirty`.

**First pairing:** empty local DB + cursor 0 → full pull. **First use with no server:** local-only, cursor unset; pairing later is a full push of local rows then pull.

## Auth (v1)

Env on the server: `APP_PASSWORD`, `SESSION_SECRET`. Optional `IGDB_CLIENT_ID` / `IGDB_CLIENT_SECRET` (search disabled until set).

- **Web:** `POST /v1/auth/login` → httpOnly session cookie. SPA has a login screen.
- **iOS:** same login with server URL + password → opaque bearer token stored in the iOS Keychain (see architecture). Subsequent calls use `Authorization: Bearer …`. The password is not saved.
- Tokens are long-lived (personal LAN app). Logout / “forget server” deletes the server-side token row and the Keychain item.
- No signup, OAuth, or per-device management UI in v1. One password in Compose env / `.env`.

## HTTP API (sketch)

All JSON, prefix `/v1`. OpenAPI file at repo root is the contract (`openapi.yaml`). Go implements it; web and iOS are hand-written clients from that spec (no heavy codegen required in v1).

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness |
| POST | `/v1/auth/login` | Password → cookie or token |
| POST | `/v1/auth/logout` | |
| GET | `/v1/library` | List (filters: `platform`, `q`) |
| GET | `/v1/library/{id}` | Detail |
| POST | `/v1/library` | Create |
| PATCH | `/v1/library/{id}` | Update |
| DELETE | `/v1/library/{id}` | Tombstone |
| POST | `/v1/sync` | Bidirectional sync |
| GET | `/v1/platforms` | Curated IGDB consoles/handhelds (+ PC) |
| GET | `/v1/search/games?q=&platform=` | Proxied IGDB search |
| GET | `/v1/search/barcode?q=` | UPC/EAN → product title + IGDB search |
| GET | `/v1/covers/{id}` | Cached image bytes |

Web CRUD uses the resource routes. iOS prefers `/sync` after first load but may use GET for a one-shot refresh.

## v1 product surface

**Both clients**

- Cover grid + list, sort by title / date added
- Filter by platform; search the local library by title
- Game detail: cover, title, platform, region, completeness, notes, dates
- Edit those fields; delete (with confirm)
- Add: search IGDB, scan/type a box barcode, or enter manually → pick platform → optional region/completeness → save
- Manual add when IGDB misses (or iOS has no server)
- Empty states and a visible sync/offline indicator on iOS

**Web-only**

- Login, served at the server origin
- Settings readout: IGDB configured yes/no (no full settings app in v1)

**iOS-only**

- Settings: server URL, password (login only, not stored), connection status, last sync time, “forget server” (drops the Keychain token)
- Local SQLite as the working copy at all times

**Explicitly not v1** (backlog)

- Bug: missing images on web after ios app syncs to initial empty server state
- Web platform sidebar — filter the library by platform with per-platform counts (iOS later)
- Statistics view — counts by platform, region, completeness, and similar shelf breakdowns
- PriceCharting / values
- Wishlist, multiple collections, hardware, amiibo
- Custom fields
- Custom cover upload, back-of-box photos
- Multi-user, owner field, sharing
- Bonjour/NAS discovery, push notifications
- Android, public App Store listing

CSV export/import, barcode scan, and bulk add are implemented. The web platform sidebar is the next named follow-up.

## Repo layout (monorepo)

```text
schema/schema.sql          # SQLite schema (source of truth)
openapi.yaml
docker-compose.yml
Dockerfile                 # multi-stage: web build → Go binary
.env.example
server/                    # Go module: api, sqlite, igdb, auth, sync
web/                       # Vite + React + TS + Tailwind
ios/                       # Xcode project, SwiftUI
```

Docker Compose: one `app` service, port `8080`, volume `./data` for SQLite + covers. Multi-stage image:

1. `npm ci && npm run build` in `web/`
2. `go build` with `go:embed` of `web/dist` (and/or copy into the image)
3. Distroless/debian-slim runtime. Prefer **pure-Go SQLite** (`modernc.org/sqlite`) so CGO is not required.

Go stack: `net/http` (Go 1.22+ mux), `sqlc`, `golang-migrate` (or goose), structured JSON logs. Web: Vite, React 19, React Router, TanStack Query, Tailwind. iOS: iOS 17+, SwiftUI, GRDB.swift, Keychain, URLSession.

## Implementation order

Each phase should be mergeable and demoable.

### Phase 0 — Skeleton

Compose, Dockerfile, `.env.example`, `schema/schema.sql`, `openapi.yaml`, README (run instructions, IGDB/Twitch key setup). Empty Go `GET /health`.

### Phase 1 — Library API

SQLite, migrations, password auth, CRUD + tombstones for `library_items`. Manual JSON create (no IGDB yet). Seed a couple of rows for demos.

### Phase 2 — IGDB proxy + covers

Platforms list, search, game detail fetch, download cover to `data/covers`. Rate-limit (≤4 req/s) and cache. `POST /v1/library` can accept `{ igdb_game_id, igdb_platform_id, region, completeness }` and snapshot title/cover.

### Phase 3 — Web app

Login, grid/list, filters, search, add-from-IGDB flow, detail/edit/delete. This is the first “it feels like CLZ” milestone: `docker compose up` → catalog a game in the browser.

### Phase 4 — Sync endpoint

`POST /v1/sync` + LWW + cursor. Go unit tests for: insert, update win/lose, tombstone, cursor pagination, clock clamp. This is the riskiest logic; test it **before** the iOS client exists.

### Phase 5 — iOS local app

SwiftUI shell, GRDB mirroring `library_items`, grid/list/detail/edit/manual add. Fully usable with no network.

### Phase 6 — iOS pairing + sync

Settings (URL + password), Keychain, sync service, cover download, online/offline banner, pull-to-refresh. Verify: add on phone offline → appear on web after reconnect; delete on web → disappear on phone; conflict fixture uses LWW.

## Verification

No browser-tooling is assumed for the Go/iOS work; use Compose + curl/HTTP and the iOS Simulator.

**Server / web**

- `docker compose up --build` serves the SPA at `:8080` and `/health` returns ok
- Wrong password is rejected; right password sets a session
- IGDB search returns covers; adding a game writes SQLite + a cover file on the volume
- Restart container: library still there (volume)
- CRUD + filter + local search in the web UI (Safari or Chrome), including empty and error states
- Sync tests as above, including two sequential “devices” simulated with curl

**iOS**

- Simulator: add/edit/delete with **server off** — data survives process kill
- Point at Compose on the LAN/host, login, full pull
- Offline mutation then reconnect: web shows the change
- Web mutation then iOS foreground: phone shows the change
- Delete both directions with tombstones (row does not resurrect on next sync)

**Cannot verify in this environment:** a physical iPhone, camera, or App Store / TestFlight. Running on a device needs a paid Apple Developer account and signing in Xcode.

## Prerequisites you will need

1. **Docker** on the NAS/homelab
2. **Twitch developer credentials** for IGDB (free): [IGDB API getting started](https://api-docs.igdb.com/#getting-started)
3. **Xcode** + (for a real iPhone) **Apple Developer** membership
4. LAN hostname or IP the phone can reach (`http://nas.local:8080` or similar)

## What “done” means for v1

You can `compose up` on the NAS, log into the web UI, search IGDB, and fill a shelf of physical games with covers. The iPhone app shows the same shelf, works on a plane, and converges automatically when you get home. Nothing else is required to call v1 shipped.

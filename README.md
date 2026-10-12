# Air-raid alert map (working title)

A web app (PWA) for Ukraine that brings air-raid alerts, power outage schedules and regional news together in one place. The interface is in Ukrainian.

Status: a live air-raid alert map with push notifications per region, an installable PWA that works offline, regional news with AI answers (Chernihiv oblast), and power outage schedules for six regions (Kyiv city; Kyiv, Dnipropetrovsk, Chernihiv, Kharkiv and Poltava oblasts).

## Layout

```
server/   Go: polls the alert source, serves the API and SSE
web/      SvelteKit (static build): the site
data/     oblast boundaries (GeoJSON) and their licenses
```

## Quick start

```sh
./run.sh   # builds the site and the server, then open http://localhost:8080
```

The script also prints a LAN address for checking the site from a phone on the same Wi-Fi.

## Development

Requires Go 1.27+ and Node 22.17+.

```sh
# terminal 1: server on :8080 (runs on test data without a token)
cd server && go run ./cmd/server

# terminal 2: site on http://localhost:5173, /api is proxied to the server
cd web && npm install && npm run dev
```

Without an alerts.in.ua token the server uses a mock source with random alerts, and the site shows a «Тестові дані» (test data) badge. For faster updates while testing: `POLL_INTERVAL=5s go run ./cmd/server`.

With a token:

```sh
ALERTS_IN_UA_TOKEN=... go run ./cmd/server
```

All settings are described in [.env.example](.env.example).

## Build

```sh
cd web && npm run build                     # static site in web/build
cd server && go build -o bin/server ./cmd/server
STATIC_DIR=../web/build ./bin/server        # the server serves both the API and the site
```

## Checks

```sh
cd server && go vet ./... && go test -race ./...
cd web && npm run check
```

## API

- `GET /api/alerts`: current state (JSON).
- `GET /api/stream`: SSE; a full snapshot on connect and after every change.
- `GET /api/power`: regions with power outage schedules.
- `GET /api/power/{uid}`: today's and tomorrow's schedule for a region (404 if unsupported, 503 before the first fetch).
- `POST /api/feedback`: anonymous feedback (JSON). Rate-limited per client and globally; no IP addresses are stored.
- `GET /api/push/key`, `POST /api/push/subscribe`, `POST /api/push/unsubscribe`: Web Push subscriptions per region. Only endpoints of known push services are accepted.
- `GET /api/news/{uid}`, `GET /api/news-item?id=`: regional news (title and preview, never the whole post).
- `POST /api/ai/news`: AI answer about a news post. The question must be `summary` or one of the follow-ups the AI suggested for that post: there is no free-form chat.
- `GET /api/stats/alerts?hours=24`: alerts per region over a period (count, total and longest minutes; overlapping district alerts count once). History is kept for 30 days.
- `GET /healthz`

## Power outage schedules

The server fetches schedules every 5 minutes (`server/internal/power/regions.go`):

- Kyiv city and Dnipropetrovsk oblast: the YASNO (DTEK) public API.
- Kyiv, Chernihiv, Kharkiv and Poltava oblasts: community datasets [outage-data-ua](https://github.com/Baskerville42/outage-data-ua) and [OE_OUTAGE_DATA](https://github.com/yaroslav2901/OE_OUTAGE_DATA), credited on the site.

## Feedback

Feedback is stored in SQLite at `server/var/app.db` (see `DATA_DIR`) and deleted after 12 months. To read it:

```sh
cd server && go run ./cmd/feedback        # latest 50 entries; -n for more
```

The in-app limits protect the form and the database; protection against a real DDoS belongs at the reverse proxy or hosting level.

## Push notifications and offline use

Users tap «Стежити» on a region to get a notification when an alert starts or ends there. The server stores only the browser-issued push endpoint and the followed regions; expired subscriptions are removed when the push service reports them gone. VAPID keys are generated on first start in `server/var/vapid.json`: keep that file, since new keys invalidate every subscription.

The service worker (`web/src/service-worker/`) caches the app shell and the last `/api/alerts` and `/api/power/*` responses, so the site opens offline with the last known data.

Push and service workers need a secure origin. `http://localhost:8080` counts as one, so everything works on the development machine. To test from an Android phone on the LAN without HTTPS, add the LAN address in Chrome at `chrome://flags/#unsafely-treat-insecure-origin-as-secure`. iPhones need real HTTPS and the site added to the home screen (iOS 16.4+).

## News and AI

News comes from a whitelist of public Telegram channels per region (`server/internal/news/service.go`), read from their public web preview every 5 minutes and kept for a week. Posts that track drones or missiles in real time, and alert/all-clear notices, are filtered out: the map shows alerts, and relaying where weapons are heading is not this service's job.

AI mode (`/ai/?news=<id>`) shows the post, a short summary and three follow-up questions suggested by the model; tapping one asks it. Answers use only the post's text (the post is passed as data, not instructions), must be in Ukrainian (answers with Russian-only letters are rejected and the next model is tried), are cached per post and question, and are capped by `AI_DAILY_LIMIT`. Models are configured with `AI_MODELS`; without `OPENROUTER_API_KEY` AI is disabled.

## Map

`web/src/lib/map/regions.ts` is generated from `data/geo/ukr-adm1.geojson` and `data/geo/land-ukr.geojson` with `cd web && npm run gen:map`. Boundaries: © OpenStreetMap contributors (ODbL).

## License

[AGPL-3.0-or-later](LICENSE).

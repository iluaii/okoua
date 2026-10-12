# Air-raid alert map (working title)

A web app (PWA) for Ukraine that brings air-raid alerts, power outage schedules and regional news together in one place. The interface is in Ukrainian.

Status: a live air-raid alert map, and power outage schedules for six regions (Kyiv city; Kyiv, Dnipropetrovsk, Chernihiv, Kharkiv and Poltava oblasts).

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
- `GET /healthz`

## Power outage schedules

The server fetches schedules every 5 minutes (`server/internal/power/regions.go`):

- Kyiv city and Dnipropetrovsk oblast: the YASNO (DTEK) public API.
- Kyiv, Chernihiv, Kharkiv and Poltava oblasts: community datasets [outage-data-ua](https://github.com/Baskerville42/outage-data-ua) and [OE_OUTAGE_DATA](https://github.com/yaroslav2901/OE_OUTAGE_DATA), credited on the site.

## Map

`web/src/lib/map/regions.ts` is generated from `data/geo/ukr-adm1.geojson` and `data/geo/land-ukr.geojson` with `cd web && npm run gen:map`. Boundaries: © OpenStreetMap contributors (ODbL).

## License

[AGPL-3.0-or-later](LICENSE).

# dashboard

React (Vite + TypeScript) live operations dashboard for the SAR drone
system. `/` (Live Map) is the primary view: a dark-themed cockpit combining
a live Leaflet map, an alert feed, connectivity/health status, and a
detection detail panel, all driven by `fusion-service`'s REST API and
`/ws/live` WebSocket.

## Live Map (`/`)

- **Map** (`components/MapView.tsx`) — Leaflet + OpenStreetMap tiles.
  Detections render as canvas circle markers colored by class: person red,
  fire orange, flood blue, structural_damage amber, debris gray (not
  specified in the original spec — given a neutral color instead of
  implying an unstated priority). `GET /mission/route-status` segments are
  drawn as polylines (red = obstructed, dashed green = clear).
- **Status bar** — connectivity state (`Live` / `Offline — Queueing` /
  `Disconnected`) derived from the most recent `/ws/live` "detection"
  message's `status` field (see `hooks/useFusionData.ts`).
- **Health strip** — detection throughput (per minute) and average fusion
  latency (ms), both computed client-side from each `detection` message's
  `detected_at` vs. its actual arrival time — no backend metrics endpoint
  needed.
- **Detail panel** — click a marker to see that detection's class,
  confidence, timestamp, and coordinates.
- **Alert feed** — `GET /alerts` seeded on load, then live-appended via the
  `"alert"`-typed `/ws/live` message; sorted newest first with
  priority-tier-colored badges (critical/high/medium).
- **Debug panel** (dev only — `import.meta.env.DEV`, never ships in a
  production build) — a "Simulate offline" button that calls
  `POST /debug/set-offline` on fusion-service, for demoing the
  offline-then-sync behavior live.

`/ws/live` messages are `{"type": "detection" | "alert", "payload": ...}`
envelopes — see `hooks/useFusionData.ts`.

## Other routes

- `/detections` — placeholder
- `/status` — polls `/health` on `inference-service` and `fusion-service`

## Local setup (no Docker)

```bash
cd dashboard
cp .env.example .env
npm install
npm run dev
```

Requires a reachable `fusion-service` (REST + `/ws/live`) at
`VITE_FUSION_URL` — see [`../fusion-service/README.md`](../fusion-service/README.md).

## Config (env vars, see `.env.example`)

| var                 | default                  |
|---------------------|---------------------------|
| `VITE_INFERENCE_URL`| `http://localhost:8001`   |
| `VITE_FUSION_URL`   | `http://localhost:8002`   |

## Manual verification

Browser-tested with a headless Playwright pass (no Go/Postgres/Redis
toolchain was available in the environment this was built in, so a small
throwaway Node mock server stood in for fusion-service's REST/WebSocket
contract — not part of this repo). Confirmed: map renders with colored
markers; live WS detections/alerts update in place without a page reload;
marker click opens the detail panel with the right fields; alert badges
render by tier; health-strip metrics update from zero; the offline-toggle
flips the status pill between `Live` and `Offline — Queueing` and back; zero
browser console errors throughout. Re-verify against the real
`fusion-service` once a full stack (Postgres/Redis/Go) is available.

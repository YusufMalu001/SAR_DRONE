# Store-and-Forward Sync Protocol & Conflict Resolution

## 1. Architecture Overview
In contested or disaster disaster environments, network connectivity between drone edge nodes and command-centre databases/cloud endpoints cannot be assumed. The SAR drone system implements an end-to-end **Store-and-Forward** architecture backed by an embedded, zero-dependency SQLite local queue (`localqueue`).

Every outbound path:
1. **Detections**: Raw fused detections.
2. **Entities**: Clustered survivor and hazard entities.
3. **Alerts**: High-priority responder alerts.
4. **Outbound Dispatch**: External webhooks, MQTT, email, and SMS notifications.

is written synchronously to the durable on-disk SQLite queue **first** before any network or remote database transmission is attempted.

---

## 2. Delivery Semantics & Idempotency

### 2.1 At-Least-Once Delivery
All records are guaranteed **at-least-once** delivery:
- Records are deleted from the local SQLite queue **only** after receiving an explicit success acknowledgement (e.g. `200 OK` from remote API, or successful commit from PostgreSQL).
- If a connection drops mid-transmission or during a crash, the items remain in the SQLite queue and are replayed upon reconnection.

### 2.2 Idempotency Keys
To prevent duplicate processing upon re-delivery:
- **Detections**: Use deterministic UUID `id` minted at detection generation.
- **Entities**: Use persistent entity UUID `id` minted at initial cluster creation.
- **Alerts**: Use unique alert UUID `id`.
- **Outbound Webhooks / MQTT**: Include `X-Idempotency-Key: <UUID>` in HTTP headers and message payloads.

### 2.3 Deterministic Conflict Resolution Rule
When replaying queued updates to PostgreSQL, conflicts are resolved deterministically using **Last-Write-Wins (LWW) with Monotonic Observation Precedence**:
- **Detections**: `ON CONFLICT (id) DO UPDATE SET track_id = EXCLUDED.track_id, modalities = EXCLUDED.modalities, priority_score = EXCLUDED.priority_score`.
- **Entities**: Updates are applied only if the incoming record's `last_seen` timestamp is greater than or equal to the currently stored `last_seen` timestamp (`WHERE entities.last_seen <= EXCLUDED.last_seen`).
- **Hazard Polygons**: Recomputed dynamically via PostGIS `ST_ConvexHull` upon receiving new hazard points.

---

## 3. Reconnect & Drain Backoff Policy

Upon detecting an offline $\to$ online transition (via PostgreSQL ping recovery or manual override clear):
1. **Drain Order**: Items are drained strictly in chronological order (`ORDER BY queued_at ASC, detected_at ASC`).
2. **Exponential Backoff**:
   - Initial interval: $500\text{ms}$.
   - Multiplier: $2.0$.
   - Max backoff: $10.0\text{s}$.
   - Random jitter: $\pm 10\%$.
3. If an individual item fails (e.g. transient DB timeout), the drain pass halts to preserve ordering; retries resume after the backoff delay.

---

## 4. Exposed Sync Metrics

The sync subsystem exposes live metrics on `GET /debug/queue`, `GET /health`, and Prometheus `/metrics`:
- **`queue_depth`**: Total number of pending items awaiting sync.
- **`oldest_item_age_seconds`**: Elapsed seconds since the oldest item in the queue was recorded.
- **`last_successful_sync`**: RFC3339 timestamp of the most recent successful queue flush.

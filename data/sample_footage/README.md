# sample_footage

- `demo_telemetry.csv` — a small (20-row, 4-second) synthetic lat/lon/alt/heading
  flight path at 5 Hz, checked into git so `TelemetryPublisher` has something
  to replay out of the box. Columns: `lat, lon, alt, heading, elapsed_s`
  (`elapsed_s` is seconds since flight start — see
  `../../inference-service/app/pipeline/telemetry_publisher.py`).

- `demo_flight.mp4` — **not included** (gitignored, too large). Drop a short
  local video clip here with that exact filename before running
  `VideoIngestionPipeline` / `python -m app.pipeline.run_demo`, or point
  `VIDEO_PATH` / `--video` at wherever your clip actually lives.

# shared

Schema definitions shared by every service in the monorepo. These are the
contract for messages published on Redis pub/sub channels (`detections`,
`telemetry`) and consumed by `fusion-service` and `dashboard`.

- `proto/detection.proto`, `proto/telemetry.proto` — canonical Protobuf
  definitions. Not yet wired into codegen; kept as the source of truth for
  field names/types across languages.
- `jsonschema/detection.schema.json`, `jsonschema/telemetry.schema.json` —
  JSON Schema mirrors of the proto messages, used by `inference-service`
  (Python) to validate outgoing messages and available to `dashboard` for
  client-side validation/typing.

## Detection

| field       | type   | notes                                                        |
|-------------|--------|---------------------------------------------------------------|
| id          | string | unique id (UUID)                                              |
| class       | enum   | `person` \| `fire` \| `flood` \| `debris` \| `structural_damage` |
| confidence  | float  | 0.0 - 1.0                                                      |
| bbox        | object | `{x, y, w, h}` normalized 0-1                                  |
| frame_id    | string | id of the source video frame                                   |
| source      | enum   | `rgb` \| `thermal`                                             |
| timestamp   | string | RFC3339                                                        |

## Telemetry

| field     | type   | notes            |
|-----------|--------|------------------|
| lat       | float  | degrees          |
| lon       | float  | degrees          |
| alt       | float  | meters           |
| heading   | float  | degrees, 0-360   |
| timestamp | string | RFC3339          |

If codegen is added later, regenerate language bindings from `proto/` and
keep `jsonschema/` in sync by hand (or generate it from the proto files).

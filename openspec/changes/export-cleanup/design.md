## Approach

This is a purely mechanical cleanup — no architectural decisions needed.
Each task is independent and can be verified in isolation.

## Implementation Details

### Proto reservation

Add two lines inside the `DescribeResponse` message block:

```protobuf
message DescribeResponse {
  reserved 6;
  reserved "supports_export";
  // ... existing fields ...
}
```

Then run `make proto` to regenerate `api/plugin/plugin.pb.go` and
`api/plugin/plugin_grpc.pb.go`.

### Config cleanup

Remove:

```yaml
collector:
  endpoint: localhost:4317
```

from `.complytime/complytime.yaml`. No struct changes needed since the
Go parser already ignores unknown keys (it was removed in PR #617).

### CRAP baseline

Run `make crapload-baseline` to regenerate `.gaze/baseline.json` with
only currently-existing functions. This removes the 17 stale entries
from deleted export infrastructure.

## Risks

- **Proto regeneration diff noise**: The generated `.pb.go` files may
  show formatting changes if the local `protoc-gen-go` version differs
  from CI. Mitigated by using `buf generate` via `make proto` which
  pins versions in `buf.gen.yaml`.
- **No risk of behavioral regression**: All three tasks are cosmetic.

## Alternatives Considered

- **Do nothing**: Acceptable short-term but accumulates debt. Field 6
  reuse risk grows as the proto evolves.
- **Delete the messages entirely from proto history**: Unnecessary.
  Proto3 reserved statements are the standard mechanism.

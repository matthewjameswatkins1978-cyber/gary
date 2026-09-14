# GARY testing

Run the local gates:

```powershell
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/gary
go run ./cmd/gary doctor
```

The Go tests cover flag order and exclusivity, durable packet/state recovery,
duplicate-safe events, two-worker ceilings, explicit candidate routes, and
independent acceptance. Telltail remains an offline decision/replay gate; a
live provider canary requires a separately bounded explicit run.

Black Box tests cover append-only journal behavior, derived note regeneration,
history rebuild after index corruption, filters, explicit related jobs,
incident suspected/proven-cause semantics and proof-required closeout, redacted
bundles, readable restart breadcrumbs, and honest insufficient-data health
inputs. Black Box is local and deterministic; it adds no paid or live AI call
to CI.

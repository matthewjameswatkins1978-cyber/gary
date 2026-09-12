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
duplicate-safe events, strict two-worker independence and ceilings, whole-job
completion, mechanical waiting while another worker is healthy, serial
integration of two candidates from one base, rollback after failed composed
acceptance, explicit candidate routes, and independent acceptance.

GitHub GARY CI runs for pull requests to `master` and for direct pushes to
`master`, so the default branch cannot silently bypass the test/vet/CLI gate.
Telltail remains an offline decision/replay gate; a live provider canary
requires a separately bounded explicit run.

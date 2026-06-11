# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`dotbomb` is a DNS stress-test CLI. It drives load against a target resolver over
five transports — plain DNS (UDP), DNSSEC, DoT, DoH (POST/GET) — plus a `-m all`
fan-out and an optional libpcap-based fake-source mode that emits spoofed
Ethernet+IPv4+UDP frames. See `README.md` for the full flag reference and example
invocations.

Naming gotcha: the Go module is `github.com/acom-networks/dnsbomb`, the binary and
tool are `dotbomb`, and the Makefile macOS target is spelled `drawin` (intentional —
do not "fix" it; the README calls this out).

## Commands

```bash
# Plain build (needs libpcap + cgo for the fake-source path)
go build -o bin/dotbomb ./cmd/dotbomb

# Cross-compile + zip into bin/ (sets CGO_ENABLED=1, injects version via -ldflags)
make linux        # linux/amd64
make windows      # windows/amd64
make drawin       # darwin/amd64

# Tests / benchmarks (all live in pkg/stress)
go test ./pkg/stress/
go test -race -run TestDNSCompletesOnSmallRun ./pkg/stress/   # single test, race-checked
go test -bench . ./pkg/stress/

go vet ./...
gofmt -l cmd/ pkg/ test/

# Run (see README for all flags)
./bin/dotbomb -m dns -c 4 -n 25 -t 2 -tps 3000 -r 8.8.8.8 -f domains.txt
```

cgo/libpcap is required only because `gopacket/pcap` (fake-source mode) needs it.
Install: `libpcap-dev` (Debian), `libpcap-devel` (RHEL), or `brew install libpcap`
(macOS). A plain `go build` of the whole module still pulls in pcap, so the dev host
needs libpcap headers present.

## Architecture

### Layout

- `cmd/dotbomb/flag.go` — CLI flag parsing, validation, and default-port selection.
  All input validation lives here (mode whitelist, fake-mode MAC auto-fill from the
  interface, `-ignore` only with `-m dns`).
- `cmd/dotbomb/main.go` — orchestration: loads/validates the domain list, builds the
  rate limiter, computes `Expected`, launches one goroutine per transport, collects
  statuses, and prints the report.
- `pkg/stress/` — the engine. One file per transport (`dns.go`, `dnssec.go`,
  `dot.go`, `doh.go`) plus `stress.go` for shared state and helpers.
- `test/` — standalone manual helper servers (a TLS listener, an HTTPS `/dns-query`
  handler, a raw pcap sender) used for hand-testing against localhost. These are
  `package main` programs, **not** Go unit tests.

### Run lifecycle and the singleton-state contract

The whole engine coordinates through **package-level globals** in `stress.go`
(`Result`, `StatusChan`, `DoneChan`, `doneOnce`). The package is therefore **not
reentrant — exactly one stress run per process.** Tests that drive a real run (e.g.
`dns_integration_test.go`) mutate these globals and rely on running alone.

The completion protocol, which spans `main.go` + every transport file:

1. `main` sets `bomb.Expected = statusN * C * T`, where `statusN` is 4 for `-m all`
   (all four transports tally into the *same* singleton `Result`) and 1 otherwise.
2. Each transport's receive path tallies outcomes into `Result`'s atomic counters.
   `Result.Processed()` = Ans + NoAns + Timeout + Other.
3. `MaybeSignalDone(Expected)` calls `SignalDone()` once `Processed() >= Expected`;
   `SignalDone` closes `DoneChan` exactly once (via `doneOnce`) so all transports in
   `-m all` unblock together.
4. Each transport goroutine reports one status into the buffered `StatusChan`
   (0 = finished, 1 = idle-timeout). `main` reads `statusN` statuses and reports the
   worst (timeout > finish); a signal (`SIGINT`/etc.) reports `Cancel`.

### Counter flushing — the load-bearing detail

Receive paths accumulate counts in **goroutine-local** variables and flush to the
shared atomics in batches, to keep atomic contention off the hot path. There is a
critical asymmetry:

- **UDP (`drainUDPReplies` in `stress.go`, shared by DNS + DNSSEC)** loops on
  `conn.Read` forever with no natural termination. It flushes on size (`flushSize`)
  **or** on a read-deadline timeout (`flushInterval`). The time-based flush is
  essential: a run receiving fewer than `flushSize` replies (the common case) must
  still publish its tallies and signal completion, otherwise the run stalls until the
  idle watchdog fires and falsely reports a timeout with zero recv counts. (This was a
  real regression — see `TestDNSCompletesOnSmallRun`.) When touching this, preserve
  the size-or-time flush.
- **DoT/DoH** issue synchronous request→response per query inside a worker loop that
  *does* terminate after `TotalRequest` iterations, so a guaranteed final flush after
  the loop publishes the last partial batch. They only need a size threshold.

### Idle watchdog

`watchIdle(done, idle)` (in `stress.go`) is the per-transport timeout. It polls
`Result.Processed()` and returns `true` only if no progress is made for `idle`
(= `-t` seconds). This replaced earlier per-goroutine `timer.Reset` calls, which were
a data race when many receivers reset one shared timer. Send-driven modes
(`-ignore`, fake) bypass the watchdog entirely and finish on `DoneChan`.

### Rate limiting

A single `golang.org/x/time/rate.Limiter` is shared across all workers. Its **burst**
must be `>= stress.MaxBatch` (the largest `n` any transport passes to `WaitN`),
because `WaitN(n)` returns immediately with an error when `n > burst`, silently
bypassing the limit. `main` sets burst to `max(stress.MaxBatch, concurrency)`. If you
add or raise a `batchSize` constant in a transport file, keep `MaxBatch` in sync.

### Send paths

Queries are packed once into `prePacked [][]byte` before the hot loop (`Pack()` is
expensive). DNS has two send paths selected by `b.FakeIF`:

- **Real socket**: `net.DialUDP` per worker with large read/write buffers; a separate
  receive goroutine runs `drainUDPReplies`.
- **Fake source** (`-finet`): each worker owns a pcap handle and serializes spoofed
  Ethernet+IPv4+UDP frames, rotating the source IP via `startingFakeIP`/`nextIPv4`
  (byte 2 offset by workerID to avoid worker collisions). No replies are read; the run
  is send-driven. A `fakeWG` watcher calls `SignalDone` if all fake workers exit early
  (bad interface/MAC) so `main` never hangs.

DoT/DoH reuse a single routedns client per worker and fan out `inflight` inner
goroutines over it — DoT pipelines on one TLS connection, DoH multiplexes via HTTP/2.
Both use `InsecureSkipVerify` by design (the tool targets arbitrary IPs).

## Conventions

- Domain list format is `domain. QTYPE` per line (e.g. `google.com. A`). `main.go`
  validates each QTYPE against `stress.QType` at load and normalizes names with
  `dns.Fqdn`; an unknown qtype is a fatal error with a line number.
- Comments in the codebase are a mix of English and Chinese; commit messages are
  often Chinese. Match the surrounding file.
- Work happens on feature branches merged into `master` (not direct commits to
  `master`).

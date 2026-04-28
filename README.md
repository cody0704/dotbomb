# DOTBOMB

DNS stress-test CLI. Supports plain DNS (UDP), DNSSEC, DoT, DoH (POST/GET), a `-m all` fan-out that hits all four transports concurrently, and an optional libpcap-based fake-source mode for spoofed-IP flood simulation.

## Build

```bash
make linux      # linux/amd64 → bin/dotbomb_<ver>_linux-amd64.zip
make windows    # windows/amd64
make drawin     # darwin/amd64  (the Makefile target name is "drawin", not a typo to fix)
```

The Makefile sets `CGO_ENABLED=1` because `gopacket/pcap` (used by the fake-source path) requires cgo. Build host needs:

- **Debian/Ubuntu** — `sudo apt install libpcap-dev build-essential`
- **RHEL/Fedora** — `sudo yum install libpcap-devel gcc`
- **Alpine** — `apk add libpcap-dev musl-dev gcc`
- **macOS** — `brew install libpcap` (system version usually works)

For a plain `go build`:

```bash
go build -o bin/dotbomb ./cmd/dotbomb
```

## Flags

```
-m         mode: dns / dnssec / dot / doh / dohg / all
-r         target server IP (required)
-p         target port (defaults: 53 for dns/dnssec, 853 for dot, 443 for doh/dohg; ignored for -m all)
-f         domain list file (required)
-c         number of concurrent workers per mode
-n         queries per worker
-t         per-query timeout (seconds)
-tps       global send rate limit (queries per second across all workers)
-inflight  in-flight queries per DoT/DoH worker (default 1; lifts per-worker throughput cap)
-ignore    skip recv goroutine, finish on send completion (dns mode only;
           use when traffic is tapped/mirrored to the target so no reply will arrive)
-v         print version

# fake-source mode (spoofed Ethernet+IPv4+UDP frames via libpcap; needs root / cap_net_raw)
-finet     network interface to send on (e.g. eth0)
-fip       starting source IP (each worker offsets byte 2 by workerID to avoid collisions)
-fsmac     source MAC
-fdmac     destination MAC (next-hop gateway)
```

`-m all` ignores `-p` and uses standard ports (53, 53, 853, 443). `-c` is per-protocol — total queries = `4 × c × n`.

## Domain list format

One `domain. QTYPE` per line, separated by a single space:

```
google.com. A
facebook.com. A
example.com. AAAA
example.org. MX
```

Supported qtypes: `A AAAA CNAME MX NS TXT SRV PTR SOA DNSKEY DS CAA NAPTR TLSA SPF ANY` (see `pkg/stress/stress.go` `QType` map).

## Examples

### DNS (UDP)

```bash
$ dotbomb -m dns -c 4 -n 25 -t 2 -tps 3000 -r 8.8.8.8 -f domains.txt
2026/04/28 13:55:16 DoTBomb start stress...
2026/04/28 13:55:16 Mode: dns
2026/04/28 13:55:16 DNS Server: 8.8.8.8:53

Run Time:	 0.039607s
Concurrency:	 4
Status:		 Finish
======================================================
Send:		 100
  LastTime:	 0.031771s
  AvgTime:	 0.000318s
  Send TPS:	 3148
Recv:		 100
  LastTime:	 0.039525s
  AvgTime:	 0.000395s
  Recv TPS:	 2530
  QType:
    Answer:	 100
    NoAnswer:	 0
    Timeout:	 0
    Other:	 0
```

### DoT

```bash
$ dotbomb -m dot -c 6 -n 20 -t 3 -tps 6000 -r 8.8.8.8 -f domains.txt
Mode: dot
DNS Server: 8.8.8.8:853

Run Time:	 0.333893s
Concurrency:	 6
Status:		 Finish
======================================================
Send:		 120
  Send TPS:	 367
Recv:		 120
  Recv TPS:	 359
  QType:
    Answer:	 120
```

### DoT with inflight pipelining

The same DoT client is reused across `inflight` inner goroutines per worker — DoT pipelines through routedns's request channel on a single TLS connection, so high inflight values lift throughput without opening more connections.

```bash
# inflight=1 (default, sequential)
$ dotbomb -m dot -c 2 -n 40 -t 5 -tps 6000 -r 8.8.8.8 -f domains.txt
Send TPS: 190    Recv TPS: 183

# inflight=8 (multiplexed)
$ dotbomb -m dot -c 2 -n 40 -t 5 -tps 6000 -r 8.8.8.8 -f domains.txt -inflight 8
Send TPS: 1111   Recv TPS: 937
```

### DoH (POST)

```bash
$ dotbomb -m doh -c 6 -n 10 -t 5 -tps 1000 -r 8.8.8.8 -f domains.txt
Mode: doh Method: POST
DoH Server: https://8.8.8.8:443/dns-query

Run Time:	 0.148052s
Concurrency:	 6
Status:		 Finish
======================================================
Send:		 60
  Send TPS:	 436
Recv:		 60
  Recv TPS:	 405
  QType:
    Answer:	 60
```

### DoH (GET)

```bash
$ dotbomb -m dohg -c 10 -n 5 -t 10 -tps 1000 -r 1.1.1.1 -f domains.txt
Mode: dohg Method: GET
DoH Server: https://1.1.1.1:443/dns-query{?dns}

Run Time:	 0.073316s
Concurrency:	 10
Status:		 Finish
======================================================
Send:		 50
  Send TPS:	 808
Recv:		 50
  Recv TPS:	 683
  QType:
    Answer:	 50
```

### `-m all` (fan out across DNS + DNSSEC + DoT + DoH)

All four transports run concurrently against the same target. `Send` and `Recv` are combined totals across all four (= `4 × c × n`).

```bash
$ dotbomb -m all -c 4 -n 25 -t 5 -tps 3000 -r 8.8.8.8 -f domains.txt
Mode: all (DNS + DNSSEC + DoT + DoH)
DNS: 8.8.8.8:53, DNSSEC: 8.8.8.8:53, DoT: 8.8.8.8:853
DoH: https://8.8.8.8:443/dns-query (POST)

Run Time:	 0.357721s
Concurrency:	 4
Status:		 Finish
======================================================
Send:		 400
  Send TPS:	 1143
Recv:		 400
  Recv TPS:	 1119
  QType:
    Answer:	 400
```

### `-ignore` (tapped/mirrored traffic)

When the test target receives traffic via a tap/mirror and won't reply on the source socket, skip the recv path entirely. Run finishes as soon as all sends are done — no idle-watchdog timeout, no `Recv` block in the report.

```bash
$ dotbomb -m dns -c 4 -n 25 -t 2 -tps 3000 -r 10.0.0.5 -f domains.txt -ignore
Run Time:	 0.031375s
Status:		 Finish
======================================================
Send:		 100
  Send TPS:	 3193
```

### Fake-source mode

Sends spoofed Ethernet+IPv4+UDP frames directly via libpcap. Each worker uses its own pcap handle; source IP byte 2 is offset by workerID so workers don't collide. Requires root / `cap_net_raw,cap_net_admin`.

```bash
$ sudo dotbomb -m dns -c 4 -n 1000 -tps 5000 -r 192.0.2.1 -f domains.txt \
    -finet eth0 -fip 10.1.0.1 -fsmac aa:bb:cc:dd:ee:ff -fdmac 00:11:22:33:44:55
```

No replies are read in fake mode — the run completes when send count reaches `c × n`.

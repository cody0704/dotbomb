# DOTBOMB

This is a DNS over TLS stress test tool

Support DNS and DoH by the way
## Command usage description

```
-v      show version
-m      mode: dns / dot / doh
-c      number of concurrency
-t      last Recv Packet Timeout
-n      number of query
-r      DNS Over TLS Server IP
-p      Port, default is 853
-f      stress domain list
```

## Example

* DoT

```bash
$ dotbomb -m dot -c 20 -n 100 -r 8.8.8.8 -f domains.txt
2022/03/26 15:44:10 DoTBomb start stress...
2022/03/26 15:44:10 Mode: dot
2022/03/26 15:44:10 DNS Over TLS Server: 8.8.8.8:853
Progress:       2000/2000

Status:          Finish
Finish Time:     2.172188s
Avg Latency:     0.001086s
==========================================
Send:            2000
Recv:            2000
  Answer:        1980
  NoAnswer:      20
  Timeout:       0
  Other:         0
  Timeout:       0
  Other:         0
```

* DNS

```bash
$ dotbomb -m dns -c 20 -n 50 -r 8.8.8.8 -f domains.txt
2022/03/26 15:44:44 DoTBomb start stress...
2022/03/26 15:44:44 Mode: dns
2022/03/26 15:44:44 DNS Server: 8.8.8.8:53
Progress:       1000/1000

Status:          Finish
Finish Time:     1.569659s
Avg Latency:     0.001571s
==========================================
Send:            1000
Recv:            1000
  Answer:        999
  NoAnswer:      0
  Timeout:       1
  Other:         0
```

* DoH

```bash
$ dotbomb -m doh -c 20 -n 300 -r 8.8.8.8 -f domains.txt
2022/03/26 15:45:10 DoTBomb start stress...
2022/03/26 15:45:10 Mode: doh
2022/03/26 15:45:10 DNS Over HTTPS Server: https://8.8.8.8:443/dns-query
Progress:       6000/6000

Status:          Finish
Finish Time:     18.190158s
Avg Latency:     0.003032s
==========================================
Send:            6000
Recv:            6000
  Answer:        5820
  NoAnswer:      180
  Timeout:       0
  Other:         0
```
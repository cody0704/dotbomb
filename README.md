# DOTBOMB

This is a DNS over TLS stress test tool

Support DNS and DoH by the way
## Command usage description

```
-v      show version
-m      mode: dns / dot / doh / dohg / dohp
-c      number of concurrency
-t      Timeout
-l      Latency interval
-n      number of query
-r      DNS Over TLS Server IP
-p      Port, default is 853
-f      stress domain list
```

## Example

* DoT

```bash
$ dotbomb -m dot -c 6 -n 120 -t 3 -l 6000 -r 8.8.8.8 -f domains.txt
2022/03/29 14:48:39 DoTBomb start stress...
2022/03/29 14:48:39 Timeout: 3s
2022/03/29 14:48:39 Latency: 0.006s
2022/03/29 14:48:39 Mode: dot
2022/03/29 14:48:39 DoT Server: 8.8.8.8:853
Progress:        120/120

Status:          Finish
Time:            0.436902s
Avg Latency:     0.003641s
==========================================
Send:            120
Recv:            120
  Answer:        120
  NoAnswer:      0
  Timeout:       0
  Other:         0
```

* DNS

```bash
$ dotbomb -m dns -c 4 -n 25 -t 2 -l 3000 -r 8.8.8.8 -f domains.txt
2022/03/29 14:49:31 DoTBomb start stress...
2022/03/29 14:49:31 Timeout: 2s
2022/03/29 14:49:31 Latency: 0.003s
2022/03/29 14:49:31 Mode: dns
2022/03/29 14:49:31 DNS Server: 8.8.8.8:53
Progress:        100/100

Status:          Finish
Time:            0.530679s
Avg Latency:     0.005307s
==========================================
Send:            100
Recv:            100
  Answer:        100
  NoAnswer:      0
  Timeout:       0
  Other:         0
```

* DoH

* Default

```bash
$ dotbomb -m doh -c 6 -n 10 -t 5 -l 1000 -r 8.8.8.8 -f domains.txt
2022/03/29 14:50:09 DoTBomb start stress...
2022/03/29 14:50:09 Timeout: 5s
2022/03/29 14:50:09 Latency: 0.001s
2022/03/29 14:50:09 Mode: doh Method: POST
2022/03/29 15:30:26 DoH Server: https://8.8.8.8:443/dns-query{?dns}
Progress:        60/60

Status:          Finish
Time:            0.192230s
Avg Latency:     0.003204s
==========================================
Send:            60
Recv:            60
  Answer:        60
  NoAnswer:      0
  Timeout:       0
  Other:         0
```

* POST

```bash
$ dotbomb -m dohp -c 5 -n 14 -t 3 -l 6000 -r 8.8.8.8 -f domains.txt
2022/03/29 14:50:46 DoTBomb start stress...
2022/03/29 14:50:46 Timeout: 3s
2022/03/29 14:50:46 Latency: 0.006s
2022/03/29 14:50:46 Mode: dohp Method: POST
2022/03/29 15:30:26 DoH Server: https://8.8.8.8:443/dns-query{?dns}
Progress:        70/70

Status:          Finish
Time:            0.460050s
Avg Latency:     0.006572s
==========================================
Send:            70
Recv:            70
  Answer:        70
  NoAnswer:      0
  Timeout:       0
  Other:         0
```

* GET

```bash
$ dotbomb -m dohg -c 100 -n 5 -t 10 -l 6500 -r 8.8.8.8 -f domains.txt
2022/03/29 15:30:26 DoTBomb start stress...
2022/03/29 15:30:26 Timeout: 10s
2022/03/29 15:30:26 Latency: 0.0065s
2022/03/29 15:30:26 Mode: dohg Method: GET
2022/03/29 15:30:26 DoH Server: https://8.8.8.8:443/dns-query{?dns}
Progress:        500/500

Status:          Finish
Time:            0.649415s
Avg Latency:     0.001299s
==========================================
Send:            500
Recv:            500
  Answer:        500
  NoAnswer:      0
  Timeout:       0
  Other:         0
```
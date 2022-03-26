# DOTBOMB

This is a DNS over TLS stress test tool

## Command usage description

```
-v      show version
-c      number of concurrency
-t      last Recv Packet Timeout
-n      number of query
-r      DNS Over TLS Server IP
-p      Port, default is 853
-f      stress domain list
```

## Source Code

```bash
go run main.go -c 20 -n 2000 -r 8.8.8.8 -f ./domains.txt
2022/03/26 14:40:44 DoTBomb start stress...
2022/03/26 14:40:45 DNS Over TLS Server: 8.8.8.8:853
Progress:       4000/4000

Status:          Finish
Finish Time:     3.472923s
Avg Latency:     0.000869s
==========================================
Send:            4000
Recv:            4000
  Answer:        3917
  NoAnswer:      80
  Timeout:       3
  Other:         0
```


## Binary executable file

### Linux / MacOS

```bash
./dotbomb_v1.1.1_darwin-amd64 -c 20 -n 100 -r 8.8.8.8 -f domains.txt
2022/03/26 14:39:49 DoTBomb start stress...
2022/03/26 14:39:49 DNS Over TLS Server: 8.8.8.8:853
Progress:       2000/2000

Status:          Finish
Finish Time:     1.084065s
Avg Latency:     0.000542s
==========================================
Send:            2000
Recv:            2000
  Answer:        1980
  NoAnswer:      20
  Timeout:       0
  Other:         0
```

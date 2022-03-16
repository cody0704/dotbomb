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
go run main.go -c 5 -n 5 -r 1.1.1.1 -f ./domains.txt
2022/03/16 13:19:56 DoTBomb start stress...
2022/03/16 13:19:56 DNS Over TLS Server: 1.1.1.1:853

Run Time:        0.176592s
Concurrency:     5
|-Status:        Finish
|-Count:         40
|-Success
| |-Send:        40
| |-Recv:        40
|   |-Answer:    40
|   |-NoAnswer:  0
|   |-LastTime:  0.146958s
|   `-AvgTime:   0.003674s
`-Error:
  `-CloseSock:   0
```


## Binary executable file

### Linux / MacOS

```bash
./dotbomb_v1.1.1_darwin-amd64 -c 1 -n 5 -r 8.8.8.8 -f domains.txt
2022/03/16 13:20:57 DoTBomb start stress...
2022/03/16 13:20:57 DNS Over TLS Server: 8.8.8.8:853

Run Time:        0.113693s
Concurrency:     1
|-Status:        Finish
|-Count:         5
|-Success
| |-Send:        5
| |-Recv:        5
|   |-Answer:    5
|   |-NoAnswer:  0
|   |-LastTime:  0.079025s
|   `-AvgTime:   0.015805s
`-Error:
  `-CloseSock:   0
```

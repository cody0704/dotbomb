# DOTBOMB

This is a DNS over TLS stress test tool

## Command usage description

```
Example:
./dotbomb -c 10 -n 100 -r 8.8.8.8 -f domains.txt
./dotbomb -c 10 -n 100 -r 8.8.8.8 -p 853 -f domains.txt

-c      number of concurrency
-t      last Recv Packet Timeout
-n      number of query
-r      DNS Over TLS Server IP
-p      Port, default is 853
-f      stress domain list
```

## Source Code

```bash
go run main.go -c 10 -n 100 -r 8.8.8.8:853 -f domains.txt

DoTBomb start stress...
Run Time:        0.232600s
Concurrency:     2
|-Status:        Finish
|-Count:         4
|-Success
| |-Send:        4
| |-Recv:        4
|   |-LastTime:  0.145471s
|   `-AvgTIme:   0.036368s
`-Error:
  |-Recv:        0
  `-CloseSock:   0
```


## Binary executable file

### Linux / MacOS

```bash
./dotbomb -c 10 -n 100 -r 8.8.8.8:853 -f domains.txt

DoTBomb start stress...
Run Time:        0.232600s
Concurrency:     2
|-Status:        Finish
|-Count:         4
|-Success
| |-Send:        4
| |-Recv:        4
|   |-LastTime:  0.145471s
|   `-AvgTIme:   0.036368s
`-Error:
  |-Recv:        0
  `-CloseSock:   0
```
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
go run main.go -c 3 -n 2 -r 8.8.8.8 -f domains.txt
2022/03/14 13:57:08 DoTBomb start stress...

Run Time:        0.093726s
Concurrency:     3
|-Status:        Finish
|-Count:         6
|-Success
| |-Send:        6
| |-Recv:        6
|   |-LastTime:  0.056690s
|   `-AvgTIme:   0.009448s
`-Error:
  |-Recv:        0
  `-CloseSock:   0
```


## Binary executable file

### Linux / MacOS

```bash
./dotbomb -c 3 -n 2 -r 8.8.8.8 -f domains.txt
2022/03/14 13:56:45 DoTBomb start stress...

Run Time:        0.082599s
Concurrency:     3
|-Status:        Finish
|-Count:         6
|-Success
| |-Send:        6
| |-Recv:        6
|   |-LastTime:  0.047309s
|   `-AvgTIme:   0.007885s
`-Error:
  |-Recv:        0
  `-CloseSock:   0
```

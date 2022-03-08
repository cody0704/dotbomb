# DOTBOMB

This is a DNS over TLS stress test tool

## Command usage description

```
Example:
./dotbomb -c 10 -n 100 -r 8.8.8.8 -f domains.txt
./dotbomb -c 10 -n 100 -r 8.8.8.8 -p 853 -f domains.txt

-c      number of concurrency
-n      number of query
-r      DNS Over TLS IP
-p      Port, default is 853
-f      stress domain list
```

## Source Code

```bash
go run main.go -c 10 -n 100 -r 8.8.8.8:853 -f domains.txt

DoTBomb start stress...
Time:   0.96s
Concurrency:     10
├─Total Query:   1000
├─Success:       1000
├─Fail:          0
└─Success Rate:  100.00%
Avg Delay:       8.653061ms
```


## Binary executable file

### Linux / MacOS

```bash
./dotbomb -c 10 -n 100 -r 8.8.8.8:853 -f domains.txt

DoTBomb start stress...
Time:   0.96s
Concurrency:     10
├─Total Query:   1000
├─Success:       1000
├─Fail:          0
└─Success Rate:  100.00%
Avg Delay:       8.653061ms
```
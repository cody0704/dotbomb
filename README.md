# DOTBOMB

This is a DNS over TLS stress test tool

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
package main

import (
	"testing"
	"time"
)

func TestQueriesPerWorker(t *testing.T) {
	tests := []struct {
		name        string
		tps         int
		d           time.Duration
		protocols   int
		concurrency int
		want        int
	}{
		{"single mode splits across workers", 1000, 2 * time.Second, 1, 4, 500},
		{"all mode also splits across 4 protocols", 1000, 2 * time.Second, 4, 4, 125},
		{"rounds up partial batches", 1000, time.Second, 1, 3, 334}, // ceil(1000/3)
		{"sub-one result floors to 1", 1, time.Second, 1, 100, 1},
		{"minutes unit", 1000, 5 * time.Minute, 1, 10, 30000}, // 300000/10
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := queriesPerWorker(tc.tps, tc.d, tc.protocols, tc.concurrency); got != tc.want {
				t.Errorf("queriesPerWorker(%d, %s, %d, %d) = %d, want %d",
					tc.tps, tc.d, tc.protocols, tc.concurrency, got, tc.want)
			}
		})
	}
}

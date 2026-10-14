// Package telemetry records bounded operation labels, never user content or credentials.
package telemetry

import (
	"encoding/json"
	"log"
	"sync"
)

var mu sync.Mutex
var counts = map[string]uint64{}

func Record(label string) { mu.Lock(); counts[label]++; mu.Unlock() }
func Snapshot() map[string]uint64 {
	mu.Lock()
	defer mu.Unlock()
	out := map[string]uint64{}
	for k, v := range counts {
		out[k] = v
	}
	return out
}
func Log() {
	value, _ := json.Marshal(map[string]any{"operation": "social_counters", "counts": Snapshot()})
	log.Print(string(value))
}

// Command vmvar measures CPU speed with workloads that match the benchmark's
// hot path, so that machines of the same type can be compared with each other.
// It prints one JSON object per measurement to stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// Same payload as app/internal/device (GET /api/devices).
type device struct {
	ID       int64  `json:"id"`
	MAC      string `json:"mac"`
	Firmware string `json:"firmware"`
}

var sample = []device{
	{ID: 1, MAC: "5F-33-CC-1F-43-82", Firmware: "2.1.6"},
	{ID: 2, MAC: "44-39-34-5E-9C-F2", Firmware: "3.0.1"},
	{ID: 3, MAC: "2B-6E-79-C7-22-1B", Firmware: "1.8.9"},
	{ID: 4, MAC: "06-0A-79-47-18-E1", Firmware: "4.0.9"},
	{ID: 5, MAC: "68-32-8F-00-B6-F4", Firmware: "5.0.0"},
}

var sink uint64

func jsonOps(d time.Duration) (ops uint64) {
	for end := time.Now().Add(d); time.Now().Before(end); {
		for range 1000 {
			b, _ := json.Marshal(sample)
			sink += uint64(len(b))
		}
		ops += 1000
	}
	return ops
}

// intOps runs a dependent integer chain (xorshift), a proxy for core clock speed.
func intOps(d time.Duration) (ops uint64) {
	x := uint64(88172645463325252)
	for end := time.Now().Add(d); time.Now().Before(end); {
		for range 1_000_000 {
			x ^= x << 13
			x ^= x >> 7
			x ^= x << 17
		}
		ops += 1_000_000
	}
	sink += x
	return ops
}

// run measures ns per operation with n goroutines working in parallel.
func run(f func(time.Duration) uint64, n int, d time.Duration) float64 {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var total uint64
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := f(d)
			mu.Lock()
			total += o
			mu.Unlock()
		}()
	}
	wg.Wait()
	return float64(d.Nanoseconds()) * float64(n) / float64(total)
}

func main() {
	reps := flag.Int("reps", 10, "repetitions of each measurement")
	dur := flag.Duration("dur", 3*time.Second, "length of one measurement")
	host := flag.String("host", "", "label for the machine")
	flag.Parse()
	enc := json.NewEncoder(os.Stdout)
	tests := []struct {
		name string
		f    func(time.Duration) uint64
		n    int
	}{{"json_1", jsonOps, 1}, {"json_2", jsonOps, 2}, {"int_1", intOps, 1}}
	jsonOps(2 * time.Second) // warm up
	for rep := 1; rep <= *reps; rep++ {
		for _, t := range tests {
			enc.Encode(map[string]any{"host": *host, "rep": rep, "test": t.name,
				"ns_per_op": run(t.f, t.n, *dur), "gomaxprocs": runtime.GOMAXPROCS(0)})
		}
	}
	fmt.Fprintln(os.Stderr, "done")
}

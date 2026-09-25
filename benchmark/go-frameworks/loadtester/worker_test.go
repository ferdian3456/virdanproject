package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/valyala/fasthttp"
)

// slots counts the requests the schedule contains, without sending anything.
func slots(cfg config, end time.Time) int {
	n := 0
	for i := range cfg.connections {
		for next := firstSlot(cfg, i); next.Before(end); next = next.Add(cfg.interval(next)) {
			n++
		}
	}
	return n
}

// counts sums the tester's completed (by status) and dropped requests.
func counts(t *testing.T, reg *prometheus.Registry) (completed map[string]uint64, dropped uint64) {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	completed = map[string]uint64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			switch f.GetName() {
			case "tester_request_duration_seconds":
				for _, l := range m.GetLabel() {
					if l.GetName() == "status" {
						completed[l.GetValue()] += m.GetHistogram().GetSampleCount()
					}
				}
			case "tester_dropped_requests_total":
				dropped += uint64(m.GetCounter().GetValue())
			}
		}
	}
	return completed, dropped
}

type result struct {
	scheduled, received int
	completed           map[string]uint64
	dropped             uint64
	lastReceived        time.Time
	end                 time.Time
}

// run drives the workers against a server that takes serverDelay per request.
func run(t *testing.T, rate float64, serverDelay, deadline time.Duration) result {
	t.Helper()
	var received atomic.Int64
	var last atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		last.Store(time.Now().UnixNano())
		time.Sleep(serverDelay)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config{url: srv.URL, method: "get", startAt: time.Now().Add(100 * time.Millisecond),
		startRPS: rate, stepRPS: 0, stages: 1, stage: time.Second, deadline: deadline, timeout: 5 * time.Second, connections: 4}
	end := cfg.startAt.Add(cfg.stage)
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	client := &fasthttp.Client{MaxConnsPerHost: cfg.connections}

	var wg sync.WaitGroup
	for i := range cfg.connections {
		wg.Add(1)
		go func() { defer wg.Done(); worker(i, cfg, client, m, end) }()
	}
	wg.Wait()
	completed, dropped := counts(t, reg)
	return result{slots(cfg, end), int(received.Load()), completed, dropped, time.Unix(0, last.Load()), end}
}

func total(c map[string]uint64) (n uint64) {
	for _, v := range c {
		n += v
	}
	return n
}

// Below capacity every scheduled request is sent once, succeeds, and reaches the server.
func TestWorkerBelowCapacity(t *testing.T) {
	r := run(t, 100, 5*time.Millisecond, 200*time.Millisecond) // capacity 4/5ms = 800 rps
	if r.dropped != 0 || total(r.completed) != r.completed["200"] {
		t.Fatalf("want only 200s, got %v and %d dropped", r.completed, r.dropped)
	}
	if int(r.completed["200"]) != r.scheduled || r.received != r.scheduled {
		t.Fatalf("scheduled %d, tester counted %d, server received %d", r.scheduled, r.completed["200"], r.received)
	}
}

// Above capacity every scheduled request is either completed or dropped, the
// tester's count of sent requests matches the server's, the server stays busy
// at its capacity (no collapse caused by the tester), and nothing is sent after
// the last deadline.
func TestWorkerOverloaded(t *testing.T) {
	deadline := 100 * time.Millisecond
	r := run(t, 1000, 20*time.Millisecond, deadline) // capacity 4/20ms = 200 rps
	if got := int(total(r.completed) + r.dropped); got != r.scheduled {
		t.Fatalf("completed %v + dropped %d = %d, want %d scheduled", r.completed, r.dropped, got, r.scheduled)
	}
	if r.dropped == 0 {
		t.Fatalf("expected dropped requests when overloaded, got %v", r.completed)
	}
	if uint64(r.received) != total(r.completed) {
		t.Fatalf("server received %d, tester sent %d (%v)", r.received, total(r.completed), r.completed)
	}
	if r.completed["200"] != total(r.completed) || r.completed["200"] < 180 {
		t.Fatalf("want about 200 successful responses in 1 s (server capacity), got %v", r.completed)
	}
	if r.lastReceived.After(r.end.Add(deadline)) {
		t.Fatalf("request received %v after the last deadline", r.lastReceived.Sub(r.end.Add(deadline)))
	}
	t.Logf("scheduled %d: completed %v, dropped %d", r.scheduled, r.completed, r.dropped)
}

// Every expected series exists with a zero value before the first request.
func TestSeriesExistBeforeFirstRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.createSeries("post")
	completed, dropped := counts(t, reg)
	for _, status := range []string{"201", "timeout", "error"} {
		if n, ok := completed[status]; !ok || n != 0 {
			t.Fatalf("status %s: want a zero series, got %v", status, completed)
		}
	}
	if dropped != 0 {
		t.Fatalf("dropped: want 0, got %d", dropped)
	}
}

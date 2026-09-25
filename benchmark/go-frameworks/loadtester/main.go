// Command loadtester is an open-model HTTP load generator in the style of
// wrk2. CONNECTIONS workers each own one keep-alive connection and send
// requests on a fixed schedule (rate / CONNECTIONS per worker), raising the
// rate stage by stage. Latency is measured from each request's scheduled send
// time, so when the server falls behind, the queueing delay is recorded
// instead of hidden (no coordinated omission).
//
// A scheduled request whose deadline (DEADLINE_MS after its scheduled time,
// like a user who gives up) passed before a worker could send it is not sent
// and is counted in tester_dropped_requests_total. Requests already in flight
// are not cancelled, because closing their connections would load the server
// with reconnects that the load generator, not the users, caused; responses
// that arrive after the deadline keep their real latency and analysis counts
// them as late. Every scheduled request is therefore accounted for exactly
// once, the backlog of an overloaded run is bounded by DEADLINE_MS, and sending
// stops about DEADLINE_MS after the last stage. TIMEOUT_MS only guards against
// a hung server.
//
// All pods of a run share START_AT (unix seconds), so their stages line up in
// wall-clock time and the aggregate rate is pods × per-pod rate.
package main

import (
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
)

const postBody = `{"mac":"EF-2B-C4-F5-D6-34","firmware":"2.1.5"}`

type config struct {
	url         string
	method      string
	startAt     time.Time
	startRPS    float64
	stepRPS     float64
	stages      int
	stage       time.Duration
	deadline    time.Duration
	timeout     time.Duration
	connections int
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloat(key, def string) float64 {
	v, err := strconv.ParseFloat(env(key, def), 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return v
}

func loadConfig() config {
	c := config{
		url:         os.Getenv("TEST_URL"),
		method:      env("REQUEST", "get"),
		startRPS:    envFloat("START_RPS", "100"),
		stepRPS:     envFloat("STEP_RPS", "100"),
		stages:      int(envFloat("STAGES", "10")),
		stage:       time.Duration(envFloat("STAGE_INTERVAL_S", "30") * float64(time.Second)),
		deadline:    time.Duration(envFloat("DEADLINE_MS", "1000") * float64(time.Millisecond)),
		timeout:     time.Duration(envFloat("TIMEOUT_MS", "5000") * float64(time.Millisecond)),
		connections: int(envFloat("CONNECTIONS", "128")),
	}
	if c.url == "" {
		log.Fatal("TEST_URL is required")
	}
	if c.method != "get" && c.method != "post" {
		log.Fatalf("REQUEST must be get or post, got %q", c.method)
	}
	c.startAt = time.Now()
	if s := os.Getenv("START_AT"); s != "" {
		sec, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Fatalf("invalid START_AT: %v", err)
		}
		c.startAt = time.Unix(sec, 0)
	}
	return c
}

// rateAt returns the per-pod target rate at time t.
func (c config) rateAt(t time.Time) float64 {
	return c.startRPS + float64(int(t.Sub(c.startAt)/c.stage))*c.stepRPS
}

// buckets returns latency buckets from 10µs to 5s, fine-grained where HTTP
// latencies usually fall (the same boundaries Anton Putra's tester uses).
func buckets() []float64 {
	var b []float64
	add := func(from, to, step float64) {
		n := int(math.Round((to - from) / step))
		for i := range n {
			// Round to 9 decimals so accumulated float error does not shift boundaries.
			b = append(b, math.Round((from+float64(i)*step)*1e9)/1e9)
		}
	}
	add(0.00001, 0.0001, 0.000005)
	add(0.0001, 0.0002, 0.000001)
	add(0.0002, 0.001, 0.00001)
	add(0.001, 0.01, 0.0005)
	add(0.01, 0.1, 0.005)
	add(0.1, 1, 0.05)
	add(1, 5, 0.5)
	return append(b, 5)
}

type metrics struct {
	duration  *prometheus.HistogramVec
	dropped   *prometheus.CounterVec
	targetRPS prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "tester",
			Name:      "request_duration_seconds",
			Help:      "Request latency measured from the scheduled send time.",
			Buckets:   buckets(),
		}, []string{"method", "status"}),
		dropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "tester", Name: "dropped_requests_total",
			Help: "Scheduled requests not sent because their deadline passed while every connection was busy.",
		}, []string{"method"}),
		targetRPS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tester", Name: "target_rps", Help: "Scheduled request rate of this pod.",
		}),
	}
	reg.MustRegister(m.duration, m.dropped, m.targetRPS)
	return m
}

// createSeries exports every expected series with a zero value before the first
// request. Managed Service for Prometheus takes a counter's first scraped value
// as its starting point, so anything counted before a series' first scrape
// (up to one scrape interval) would otherwise be lost. Statuses that are not
// created here (e.g. 500) can still lose their first scrape interval.
func (m *metrics) createSeries(method string) {
	success := "200"
	if method == "post" {
		success = "201"
	}
	for _, status := range []string{success, "timeout", "error"} {
		m.duration.WithLabelValues(method, status)
	}
	m.dropped.WithLabelValues(method)
}

func main() {
	cfg := loadConfig()
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.createSeries(cfg.method)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		log.Fatal(http.ListenAndServe(":"+env("METRICS_PORT", "8085"), mux))
	}()

	client := &fasthttp.Client{
		MaxConnsPerHost: cfg.connections,
		// One attempt per scheduled request: fasthttp retries idempotent requests by
		// default, which would send more requests than the tester records.
		MaxIdemponentCallAttempts:     1,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
	}

	log.Printf("start_at=%s start_rps=%g step_rps=%g stages=%d stage=%s connections=%d url=%s method=%s",
		cfg.startAt.Format(time.RFC3339), cfg.startRPS, cfg.stepRPS, cfg.stages, cfg.stage, cfg.connections, cfg.url, cfg.method)
	time.Sleep(time.Until(cfg.startAt))

	end := cfg.startAt.Add(time.Duration(cfg.stages) * cfg.stage)
	go func() {
		for t := cfg.startAt; t.Before(end); t = t.Add(cfg.stage) {
			m.targetRPS.Set(cfg.rateAt(t))
			log.Printf("stage=%d target_rps=%g", int(t.Sub(cfg.startAt)/cfg.stage), cfg.rateAt(t))
			time.Sleep(time.Until(t.Add(cfg.stage)))
		}
		m.targetRPS.Set(0)
	}()

	var wg sync.WaitGroup
	for i := range cfg.connections {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(i, cfg, client, m, end)
		}()
	}
	wg.Wait()

	log.Print("test finished; keeping metrics up for 60s")
	time.Sleep(60 * time.Second)
}

// worker sends one request at a time on its own schedule. If a response
// arrives late, the next scheduled time has already passed and the request
// is sent immediately; its latency still counts from the scheduled time.
// Requests whose deadline (scheduled time + deadline) passed before they could
// be sent are dropped and counted, so an overloaded worker stays current.
func worker(i int, cfg config, client *fasthttp.Client, m *metrics, end time.Time) {
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(cfg.url)
	if cfg.method == "post" {
		req.Header.SetMethod(fasthttp.MethodPost)
		req.Header.SetContentType("application/json")
		req.SetBodyString(postBody)
	}

	observers := map[string]prometheus.Observer{}
	observe := func(status string, d time.Duration) {
		o, ok := observers[status]
		if !ok {
			o = m.duration.WithLabelValues(cfg.method, status)
			observers[status] = o
		}
		o.Observe(d.Seconds())
	}
	dropped := m.dropped.WithLabelValues(cfg.method)

	for next := firstSlot(cfg, i); next.Before(end); next = next.Add(cfg.interval(next)) {
		if d := time.Until(next); d > 0 {
			time.Sleep(d)
		}
		if !time.Now().Before(next.Add(cfg.deadline)) {
			dropped.Inc()
			continue
		}
		status := "error"
		err := client.DoTimeout(req, res, cfg.timeout)
		switch {
		case err == nil:
			status = strconv.Itoa(res.StatusCode())
		case errors.Is(err, fasthttp.ErrTimeout):
			status = "timeout"
		}
		observe(status, time.Since(next))
	}
}

// interval is the gap between two requests of one worker at time t.
func (c config) interval(t time.Time) time.Duration {
	return time.Duration(float64(time.Second) * float64(c.connections) / c.rateAt(t))
}

// firstSlot staggers the workers evenly across the first interval.
func firstSlot(c config, i int) time.Time {
	return c.startAt.Add(c.interval(c.startAt) * time.Duration(i) / time.Duration(c.connections))
}

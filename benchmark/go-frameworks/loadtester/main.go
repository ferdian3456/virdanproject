// Command loadtester is an open-model HTTP load generator in the style of
// wrk2. CONNECTIONS workers each own one keep-alive connection and send
// requests on a fixed schedule (rate / CONNECTIONS per worker), raising the
// rate stage by stage. Latency is measured from each request's scheduled send
// time, so when the server falls behind, the queueing delay is recorded
// instead of hidden (no coordinated omission).
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
		targetRPS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tester", Name: "target_rps", Help: "Scheduled request rate of this pod.",
		}),
	}
	reg.MustRegister(m.duration, m.targetRPS)
	return m
}

func main() {
	cfg := loadConfig()
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		log.Fatal(http.ListenAndServe(":"+env("METRICS_PORT", "8085"), mux))
	}()

	client := &fasthttp.Client{
		MaxConnsPerHost:               cfg.connections,
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

	interval := func(t time.Time) time.Duration {
		return time.Duration(float64(time.Second) * float64(cfg.connections) / cfg.rateAt(t))
	}
	// Stagger workers evenly across the first interval.
	next := cfg.startAt.Add(interval(cfg.startAt) * time.Duration(i) / time.Duration(cfg.connections))

	for next.Before(end) {
		if d := time.Until(next); d > 0 {
			time.Sleep(d)
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
		next = next.Add(interval(next))
	}
}

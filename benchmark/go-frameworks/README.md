# Go HTTP Framework Benchmark

This directory benchmarks six Go HTTP stacks on Google Cloud: `net/http` (stdlib), chi, gin, echo, fiber and fasthttp. It measures throughput, latency percentiles (p50, p90, p99, p99.9), error rate and CPU usage under a constant request rate, and it finds the highest rate each framework can sustain within a latency SLO.

The approach is adapted from Anton Putra's benchmarks (github.com/antonputra/tutorials), with one deliberate change: load is generated at a **constant arrival rate** with wrk2 instead of a closed-loop client. A closed-loop client waits for each response before sending the next request, so it slows down together with the server and under-reports tail latency (coordinated omission). wrk2 schedules requests at a fixed rate and corrects for this, which makes the p99 numbers trustworthy near saturation.

## Layout

| Path | Purpose |
|---|---|
| `app/` | Separate Go module with one `main.go` per framework under `cmd/<framework>`. |
| `app/internal/device/` | Handler logic shared by all frameworks: JSON encoding of a device list and decoding/validation of a new device. |
| `gcp/up.sh` | Builds the binaries, uploads them to GCS and creates the two VMs. The benchmark starts on boot. |
| `gcp/sut-startup.sh` | Startup script of the system-under-test VM: runs all six servers and samples their CPU usage. |
| `gcp/loadgen-startup.sh` | Startup script of the load generator VM: builds wrk2, runs `bench.sh` and uploads results. |
| `gcp/bench.sh` | The step-load test matrix. It can also run locally against any host. |
| `gcp/wait.sh` | Waits for a run to finish and downloads its results into `results/<RUN_ID>`. |
| `gcp/down.sh` | Deletes both VMs. |
| `analyze.py` | Produces `summary.csv` and `summary.md` from a downloaded run. |

## Endpoints

Every framework exposes the same routes. All of them use `encoding/json` through the shared `device` package, so the comparison isolates routing and request/response handling.

| Route | Behavior |
|---|---|
| `GET /api/devices` | Returns a JSON array of five devices, encoded on every request. |
| `POST /api/devices` | Decodes `{"mac": "...", "firmware": "..."}`, validates it, assigns an ID and returns the device with `201`. |
| `GET /healthz` | Returns `OK`. |

No middleware (logging, recovery) is enabled in any framework. Ports are 8081 (stdlib), 8082 (chi), 8083 (gin), 8084 (echo), 8085 (fiber) and 8086 (fasthttp).

## Test design

- **Infrastructure:** one `n2-highcpu-4` VM runs the servers and one `n2-highcpu-16` VM generates load (C2D was out of stock in every us-central1 zone when this was written), both in `us-central1-a` and connected over the internal network. The load generator has four times the CPU of the server VM so that it does not become the bottleneck.
- **Isolation:** all six servers run at the same time on the server VM, but only one receives load at any moment. Idle Go servers consume negligible CPU. This keeps the hardware identical for every framework.
- **Steps:** for each framework the target rate increases through `RATES` (default 10k, 20k, 40k, 60k, 80k, 100k, 125k, 150k and 200k RPS). Each step lasts 40 seconds. wrk2 uses the first ~10 seconds for calibration, so about 30 seconds are recorded.
- **Stop condition:** the ramp stops for a framework at the first step where p99 exceeds `P99_SLO_MS` (default 50 ms), the achieved rate is below 95% of the target, or more than 1% of requests fail.
- **Fairness:** each framework gets a 15-second warm-up before its ramp, there is a 10-second cool-down between steps, the framework order is shuffled on every repetition, and the whole matrix is repeated `REPEATS` times (default 3). The summary reports medians across repetitions.
- **CPU:** the server VM records the cumulative CPU time of each server process every second, and `analyze.py` converts it into average cores used during the recorded part of each step.

## Running

Prerequisites: an authenticated `gcloud` CLI, Go, and a GCP project with billing enabled. Defaults live in `gcp/config.sh` and can be overridden through environment variables.

```bash
cd benchmark/go-frameworks/gcp

# Smoke run (a few minutes) to validate the pipeline.
FRAMEWORKS="stdlib fiber" RATES="5000 10000" REPEATS=1 ./up.sh

# Full run.
./up.sh

./wait.sh <RUN_ID>          # blocks until DONE, then downloads results
./down.sh                   # always delete the VMs afterwards
python3 ../analyze.py ../results/<RUN_ID>
```

Supported overrides passed to the load generator: `FRAMEWORKS`, `SCENARIOS` (`get`, `post`), `RATES`, `REPEATS`, `DURATION_S`, `CONNECTIONS` and `P99_SLO_MS`. Values must not contain commas.

`bench.sh` can also run locally, for example against servers started from `app/`:

```bash
SUT_HOST=127.0.0.1 WRK=/path/to/wrk2/wrk RATES="2000 4000" REPEATS=1 THREADS=2 CONNECTIONS=20 ./bench.sh
```

## Cost control

- Delete the VMs with `down.sh` after every run. A full run can take several hours, and the VMs keep billing while they exist.
- The results bucket is kept and costs very little. Delete it with `gcloud storage rm -r gs://<bucket>` when it is no longer needed.

## Limitations

- The results describe a 4-vCPU server with a trivial handler. They show per-request framework overhead, not the behavior of an application whose time is dominated by I/O or business logic.
- A single outlier step ends a framework's ramp for that repetition. Repetitions and medians reduce, but do not eliminate, this effect.
- Only HTTP/1.1 is tested, because wrk2 does not support HTTP/2.

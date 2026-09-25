# Data validation: gke-20260925-003821

This document records how the numbers in `summary.md`, `stages.csv` and the Grafana dashboard were checked against sources that do not depend on the load tester, which problems the checks found, and what the data can and cannot be used for.

The raw output of the automated checks is in `validation.txt` (produced by `gke/validate.py`).

## Verdict per metric

| Metric | Source | Independent check | Result | Verdict |
|---|---|---|---|---|
| Achieved RPS | Tester counter `tester_request_duration_seconds_count` | Packets on the app pod's `eth0` (cAdvisor, kernel counters) | 0.97–1.01 packets per request on a flat (saturated) rate; whole-run totals for fiber and fasthttp match to 0.03% | **Valid**, within about ±2% |
| p99 latency | Tester histogram | None available | See limitations 1 and 2 | **Valid below saturation**; misleading above it |
| Errors / availability | Tester status label | Definition review | Zero non-2xx responses, because the tester never gives up on a request | **Misleading** (see limitation 3) |
| App CPU | cAdvisor `container_cpu_usage_seconds_total` | GKE system metric `kubernetes_io:container_cpu_core_usage_time` and the CFS quota (verified as 200000/100000 µs = 2 CPUs) | Sources agree within ±0.07 cores over 270–300 s windows; saturated apps sit at 1.94–2.06 cores | **Valid over windows of 150 s or more** (about ±0.1 core per stage in `stages.csv`); 1–2 min windows, as in the dashboard, read up to 2.38 cores, which the quota makes impossible |
| App memory | cAdvisor working set | GKE system metric `kubernetes_io:container_memory_used_bytes` | Identical, 28–31 MiB | **Valid** |
| CPU throttling | cAdvisor CFS counters | Consistency with saturation | 0% below the knee; 18–26% at the first failing stage | **Valid** |
| Load generator headroom | GKE system metrics | — | Busiest tester pod used 0.67 of 1 CPU and was never throttled; the clients node peaked at 5.8 of 16 cores | The tester was **not** the bottleneck |

## How the RPS check works

Each GET or POST request and each response fits in a single TCP packet, so the number of packets the app pod receives is an independent count of the requests that actually reached it.

The bytes per packet confirm this. The app received 135–140 B per packet for GET and 234–239 B for POST. It sent 446–464 B per packet for GET and 238–257 B for POST. These sizes match the payloads measured locally with curl: a GET request of about 85 B, a GET response of 380 B, a POST request of about 180 B and a POST response of 166 B, each plus 52 B of TCP/IP headers. The bytes per packet are identical in both halves of the ramp, so saturation did not change how requests were carried.

| Window | Packets per tester-counted request |
|---|---|
| Stages 10–19, saturated frameworks (flat rate) | 0.980–1.010 |
| Stages 0–9 (rising rate) | 0.93–0.98 |
| Whole run, fiber and fasthttp (idle at both edges) | 1.0003–1.0007 |

The lower ratio while the rate rises comes from cAdvisor itself. Its sample timestamps lag the real measurement by about 10–15 s, so a window on a rising rate misses part of the increase at its end. On a flat rate or across idle edges the lag cancels out, and the counts match. The whole-run totals also equal the schedule: 5k to 100k RPS in 30 s stages is 31.5 million requests.

For the saturated frameworks, the whole-run comparison is not clean because of limitation 3: the tester was still sending at full speed when the window ended, and the cAdvisor lag applies again.

## Problems found and fixed during validation

1. **`analyze.py` mixed scenarios.** After a tester pod is deleted, GMP keeps returning its last value, unchanged, for up to 5 minutes. The POST analysis summed the GET testers' flat series. When those series expired in stage 3 or 4, the counter delta turned negative, and one stage per POST run lost its p99 (fiber POST stage 3 also showed a zero RPS). The tester queries now filter on `method`. The headline results did not change: the maximum rate within the SLO is the highest passing stage, and none of the affected stages were near that limit.
2. **Empty query results were summed as zero.** GMP occasionally returns a successful but empty result. Both scripts now treat that as missing data instead of zero, and `analyze.py` stops if a run has no tester data. `promql.sh` now retries transient connection failures.
3. **Per-stage CPU in `analyze.py` was biased and too noisy.** The first version took the slope between the first and last distinct cAdvisor samples in a 60 s window, which produced up to 2.8 cores against the 2-core quota. On the 73 saturated stages, whose true value must be close to 2.0, that method read 2.04 ± 0.08 cores even in a 150 s window. Linear interpolation of the counter at the window edges read 1.97 ± 0.06 (GKE system metrics: 1.98 ± 0.08). `analyze.py` now interpolates over a 150 s window centred on each stage (maximum 2.17 cores across all 240 stages). Throttling keeps the stage's own 30 s window, because it is a ratio of two counters refreshed together and the lag cancels out.
4. **The first version of the network check matched tester pods too.** The regex `<fw>-[a-z0-9]+-[a-z0-9]+` also matches `<fw>-get-xxxxx`. It now matches Deployment pods only. The CPU, memory and throttling queries in the dashboard and in `analyze.py` were never affected, because they also filter on `container="app"`.

After these fixes, every one of the 240 stage rows has data. Every stage that passes the SLO achieved its target within 1%. Each framework has exactly one pass-to-fail transition.

## Known limitations (not fixed)

1. **p99 is capped at 5 s.** The histogram's largest bucket is 5 s, so a p99 of "5 s" means "5 s or more". Beyond saturation, the real values reached minutes.
2. **Latency above saturation reflects the backlog, not the server.** Latency is measured from each request's scheduled send time (coordinated omission corrected, like wrk2). Once the offered rate exceeds capacity, the tester falls behind and the backlog keeps growing, so p99 grows with how long the test has been overloaded. Below saturation, p99 is a property of the server. Above it, only "the SLO is broken" is meaningful. Histogram buckets are 0.5 ms wide between 1 and 10 ms and 5 ms wide between 10 and 100 ms, so values such as "1.48 ms" mean "between 1.0 and 1.5 ms".
3. **The tester drains its backlog after the test ends, and availability hides overload.** The worker loop stops on the *scheduled* time (`next.Before(end)`), not on wall-clock time. A saturated tester keeps sending at full speed after the last stage; for echo GET that was 52.5k RPS for 90 s after the target dropped to 0. It stopped only because `run.sh` deleted the Job, and the unsent requests were not recorded anywhere. Because the tester never gives up on a request (the 5 s timeout counts from the actual send), availability stays at 100% even when half of the offered load is served minutes late. This does not affect the in-window achieved RPS or the maximum rate within the SLO: the backlog only exists after saturation.
4. **One repetition with 5k steps.** Differences of one step are within noise. For example, gin POST failed at 45k with p99 67 ms, and fasthttp POST failed at 70k with p99 56 ms. Both still achieved their target rate. Ranking frameworks that are one step apart needs repeated runs.
5. **Dashboard-specific issues.** The CPU panel uses a 1-minute window and shows values above 100%. The Availability panel inherits limitation 3. The RPS panel uses `rate()` and has no target line, so the backlog drain after the end of a run looks like normal traffic. The legend also contains a "Value" series from the step 4 smoke test, whose tester had no `framework` label.

## What the data supports

- Achieved throughput per stage and the maximum rate within the SLO (p99 ≤ 50 ms, achieved ≥ 95% of target, errors ≤ 1%) for each framework under a 2-CPU limit.
- The saturation points: net/http-based frameworks at about 50k GET and 45k POST; fasthttp-based frameworks at about 80k GET and 65–70k POST.
- CPU per request below saturation, averaged over several minutes.

It does not support latency comparisons above saturation, availability claims, or rankings between frameworks that differ by one step.

# Go HTTP frameworks on GKE: final results

This report answers one question: **how many requests per second can each Go HTTP framework serve within a 50 ms p99 SLO when limited to 2 CPUs?** It covers five repetitions and states only what the data supports. How the numbers were checked is in [Validation](#validation). The general method is in [`docs/benchmark-methodology.md`](../../docs/benchmark-methodology.md).

## Setup in brief

- **Apps:** one pod per framework with a 2-CPU / 256 MiB limit, on n2-standard-4 nodes (Intel Cascade Lake, SMT on). All frameworks share the same handler code: JSON encoding for GET, JSON decoding and validation for POST.
- **Load:** an open-model tester (tester v3). Requests are evenly spaced, latency is measured from the scheduled send time, and requests are dropped after a 1 s deadline.
- **Ramp:** 5k to 100k RPS in 5k steps of 30 s each, with 5 tester pods per framework.
- **Repetitions:** 5.
  - Repetitions 1–3 used random pairs of frameworks.
  - Repetitions 4–5 were a crossover: fixed pairs on pinned nodes, each pair in both node orders.
- **SLO:** p99 ≤ 50 ms, achieved RPS ≥ 95% of the target, and errors (non-2xx, timeouts and dropped requests) ≤ 1% of the target.

## Results

### 1. Two clear tiers (robust)

| Tier | Frameworks | Max rate within SLO (median of 5) GET / POST | Peak achieved RPS (median) GET / POST |
|---|---|---|---|
| fasthttp-based | fasthttp, fiber | 75–80k / 65–70k | 80–85k / 72–78k |
| net/http-based | echo, stdlib, chi, gin | 45–50k / 40–45k | 52–55k / 46–48k |

The fasthttp-based frameworks serve about 50–65% more requests per second on the same 2 CPUs: fiber about 1.5× and fasthttp about 1.6× the net/http median. This holds in every repetition, on both nodes and for both scenarios. For example, fasthttp vs. gin measured in both node orders gives a node-free ratio of 1.59 for GET and 1.66 for POST.

**Why:**
- The limiting resource is the 2-CPU quota. CFS throttling rises from 0% to about 30% exactly at each framework's knee.
- Below the knee (20k–40k RPS), the net/http frameworks use about 0.040–0.046 ms of CPU per request, against about 0.026–0.031 ms for fasthttp. That is roughly 1.6× the CPU per request.

### 2. Within the fasthttp tier: fasthttp is about 4–5% faster than fiber (significant)

- Crossover, node-free ratio: 1.049 (GET) and 1.043 (POST). The significance thresholds are 3.6% and 2.6%.
- This is consistent with fiber being a framework layer on top of fasthttp.
- An earlier reading of "80k vs 75k" came partly from fiber landing on the slower node in all three random repetitions.

### 3. Within the net/http tier: differences of at most about 3%, mostly not significant

| Comparison (crossover) | GET | POST |
|---|---|---|
| chi vs stdlib | 0.985 (not significant) | 0.980 (not significant) |
| echo vs gin | 1.033 (not significant, threshold 3.6%) | 1.032 (significant, threshold 2.6%; only one run per node order) |

- On each node separately, the order of peak achieved RPS was the same: echo ≥ stdlib > chi ≈ gin. The spread from first to last was 2.6–3.4%.
- **Conclusion:** echo, stdlib, chi and gin perform within about 3% of each other.
- **Not supported by the data:** "echo is faster than gin" with more than marginal confidence, and any strict ranking of stdlib, chi and gin.
- The median "max rate within SLO" of gin (45k GET, 40k POST) is one 5k step below the others. That is partly because gin ran on the slower node in 3 of 5 repetitions. The node-balanced peak comparison puts it within about 3% of the rest of its tier.

### 4. Latency below saturation

- Below each framework's knee, p99 stays within a few milliseconds. At 40k RPS it is about 4 ms for fasthttp/fiber and about 9 ms for the net/http frameworks.
- p50 is 0.7 ms at low load.
- These absolute latencies include the tester's own scheduling on 1 CPU, which is the same for all frameworks.
- Above the knee, requests queue until the 1 s deadline and availability collapses. That is the expected behavior of an overloaded FIFO system, not a framework property.

### 5. The node matters as much as the framework within a tier

- The two app nodes are identical on paper: same machine type, CPU platform, kernel and runtime. Yet every framework reached about 5% higher peak throughput on one of them (4% for POST).
- Separate experiments show this is not CPU core speed: both nodes encode JSON at the same speed, within 0.06%. It is not network latency either: low-load latency is within 6 µs.
- Fresh VMs of one type differ by up to about 8% in end-to-end HTTP capacity, while repeated runs on one VM differ by only 1–2%. See `results/e2evar-*`.
- **Consequences:**
  - Rankings within a tier are only valid when the candidates run on the same machines (blocking), as the crossover does.
  - Absolute numbers carry a VM-dependent uncertainty of a few percent.

## Validation

`validation.txt` checks every run (60 framework × scenario × repetition curves):

- **Accounting.** Completed plus dropped requests equal the exact number of scheduled requests (ratio 1.0000).
- **Requests vs. kernel packets.** Requests sent by the tester are compared with packets received by the app pod's network interface.
- **CPU.** App CPU from cAdvisor is compared with GKE system metrics and with the 2-core quota.
- **Tester headroom.** Tester CPU is checked to rule out the load generator as the bottleneck.

| Check | Result |
|---|---|
| Accounting | 60/60 runs exactly 1.0000. |
| Requests vs. kernel packets | 57 runs at 1.0001–1.0002. Three runs (chi GET rep 3, echo GET rep 5, stdlib POST rep 5) show 0.75–0.84% more received packets. In each of them the extra packets are small: bytes per received packet fall from 135/136/237 to 133.6/134.5/233.6 in the first half of the ramp, while response packets per request stay normal. They are TCP control packets, not extra requests. |
| App CPU | cAdvisor and GKE system metrics agree within 0.15 cores over 5-minute windows. The highest value was 2.06 cores against a 2-core quota, within sampling error. |
| Load generator headroom | The busiest tester pod used 0.63 of its 1 CPU (2 windows lacked system-metric samples). The testers were never throttled. |
| Pipeline artifacts | The problems found in the first run (series dropped before their first scrape, stale series, `rate()` extrapolation, cAdvisor timestamp lag) are handled by tester v3 and the analysis scripts. See `../gke-20260925-003821/validation.md`. |

## Limitations

- **Hardware and handler.** The results describe a 2-CPU container with a trivial handler (no database or I/O). They measure per-request framework overhead, not the behavior of real applications.
- **Two nodes.** Conclusions about the tiers are robust. Within-tier conclusions are conditional on the crossover design and on these two nodes.
- **Arrival process.** Arrivals are evenly spaced. Poisson arrivals raise p50, as shown in a local experiment. Their effect on p99 was within noise there, but it was not measured on GKE.
- **SMT.** SMT was on. A 2-CPU limit on a 4-vCPU SMT node may run on two hyperthreads of one core. An experiment showed that this costs about 25% for parallel work and adds variance.
- **Resolution.** "Max rate within SLO" is quantized to 5k steps. Use peak achieved RPS and the crossover ratios for finer comparisons.
- **Different model from Anton Putra's videos.** Those use a closed-loop load, so their latency numbers are not comparable with these.

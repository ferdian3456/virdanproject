# How to benchmark correctly

This document summarizes what the performance-evaluation literature says about running a trustworthy benchmark. For each point it records what this project does and what it does not yet do. It is written to be reused for other comparisons, such as databases, languages or runtimes, not only Go HTTP frameworks.

Sources are listed at the end. Where this document states a finding from a source, it was checked against the source's text.

## 1. Start from the question and the metric

A benchmark answers one question. Here the question is: *"How many requests per second can each framework serve within a latency SLO when given 2 CPUs?"* The question fixes the metric (the maximum rate within the SLO, plus the peak achieved rate), the resources (a 2-CPU container) and what is held constant (the handler code, which is shared by all frameworks).

- Brendan Gregg's checklist starts with "Does it matter?". The trivial handler used here measures framework overhead, not the behavior of a real application.
- The SPEC Research Group's principle P3 (*experimental setup description*) requires stating "the objective of each experiment".

## 2. Choose the workload model deliberately

**Open vs. closed load.** In a closed system a new request is sent only after a response arrives. In an open system requests arrive on their own schedule. Schroeder et al. (NSDI 2006) show that "for a given load, mean response times are significantly lower in closed systems than in open systems" (their Principle (i)).

- Anton Putra's load tester is closed-loop, and this project's tester is open-model. Their latency numbers cannot be compared directly with ours.

**Arrival process.** Lancet (USENIX ATC 2019) and Treadmill (ISCA 2016) list "the inter-arrival request distribution does not match the production environment" as a pitfall. Real traffic is usually modelled as a Poisson process, which has random gaps between requests.

- This project's tester spaces requests evenly, like wrk2. Simple queueing theory predicts that even spacing produces less queueing, and therefore lower latency, than Poisson arrivals at the same average rate.
- **The prediction did not hold for p99 when tested locally.** The experiment used the stdlib app on 1 core with a tester variant drawing exponential gaps. Constant and Poisson arrivals ran interleaved in random order, 3 rounds, 15 s per trial:
  - **p50 behaved as predicted.** Poisson was higher: 0.52 vs 0.24 ms at 20k RPS.
  - **p99 did not.** Evenly spaced arrivals gave the *higher* p99 at 28k and 33k RPS in all three rounds (38–62 vs 16–33 ms, and 93–234 vs 36–69 ms).
  - The cause is unknown. A plausible but untested explanation is synchronization: after a stall, all evenly scheduled workers are late at the same moment.
- **Conclusion:** the arrival process changes the results, but its direction for the tail depends on the system and must be measured, not assumed. The GKE results are valid for evenly spaced arrivals only.

**Enough concurrency.** Lancet notes that "the number of connections × the number of outstanding requests must be larger than the bandwidth-delay product" or the tester silently becomes closed-loop.

- Here, 640 connections per framework with one request each far exceed the requests in flight below saturation. That is about 100 at 50k RPS with 2 ms latency.
- At saturation the connection pool is what bounds concurrency. The DEADLINE_MS drop rule then keeps the accounting honest.

## 3. Measure latency correctly

- **No coordinated omission (Gil Tene).** Latency is measured from each request's *scheduled* send time. A tester that measures from the actual send time hides exactly the queueing that users feel.
- **Never average percentiles.** Per-pod histograms are summed bucket by bucket, and the percentile is computed from the merged histogram.
- **Know the histogram's resolution and cap.** Buckets are 0.5 ms wide between 1 and 10 ms and 5 ms wide between 10 and 100 ms. The top bucket is 5 s. The 1 s deadline keeps real values below the cap.
- **Tail latency needs enough samples.** Lancet points out that confidence intervals for a percentile assume independent samples, while queueing makes consecutive latencies dependent. Across-run repetition (section 6) is the safer basis for confidence.

## 4. Make sure the load generator is not the bottleneck

"Client bias" means the load generator itself adds latency or cannot reach the target rate (Lancet, Treadmill). This project checks:

- **Tester CPU per pod.** The maximum was 0.63 of the 1-CPU limit, and the testers were never CFS-throttled.
- **Client node CPU.** The maximum was 5.8 of 16 cores.
- **Target reached.** Achieved RPS equals the target in every stage that passes the SLO.

The remaining client bias is Go scheduling inside the tester. It is the same for every framework, so it does not change comparisons, but it is part of the absolute latency.

## 5. Actively validate the numbers

Brendan Gregg's *active benchmarking* means analysing the system while the benchmark runs rather than trusting the tool. His checklist questions include:

| Question | How this project answers it |
|---|---|
| Why not double? (what is the limit) | The app's 2-CPU quota. CPU throttling rises exactly at the SLO knee. |
| Did it break limits? | App CPU is checked against the 2-core quota. Short-window cAdvisor readings above 100% were traced to timestamp lag, not real usage. |
| Did it error? | Every scheduled request is accounted for as a response (by status) or as dropped. |
| Does it reproduce? | Three repetitions plus a node-crossover design (section 6). |
| Did it even happen? | Requests counted by the tester are compared with packets received by the app's network interface (kernel counters). They agree to 0.01–0.02%, except one run with 0.75% extra small packets: TCP ACKs, not requests, because the app's responses per request were normal. |

**Validate the monitoring pipeline too.** This project found four ways the metrics pipeline itself distorts data:

1. Managed Prometheus drops counts made before a counter's first scrape.
2. Deleted pods' series stay visible, flat, for 5 minutes.
3. `rate()` extrapolation over-reads at low rates.
4. cAdvisor sample timestamps lag the real measurement by 10–15 s.

Each was found only by comparing the pipeline's output with an independent source.

## 6. Control variability

Cloud performance varies over time and between machines of the same type. The SPEC Research Group's principle P1 (*repeated experiments*) says to identify the sources of variability first, then decide how many repetitions are needed, and to quantify confidence in the result.

**Find the level where variation lives (Kalibera & Jones, ISMM 2013).** Repetition "must be done at the highest level that has random variation to avoid bias".

- Here that level is the VM instance. The two app nodes have the same machine type, CPU platform, kernel, OS image and container runtime, yet one is about 5% faster: it uses about 3% less CPU per request.
- With only two VMs, conclusions hold for these two machines. Generalizing to "n2-standard-4" would require repeating on freshly created nodes.

**Randomize and interleave (Abedi & Brecht, ICPE 2017).** Running all trials of A and then all of B can report false differences, even between identical alternatives. Randomized Multiple Interleaved Trials (RMIT) runs every alternative once per round, in a new random order each round. It is a randomized block design with rounds as blocks.

- This project shuffles the pairs in every repetition.

**Block the factors you know about.** Random placement left fiber on the slower node in all three repetitions. The crossover repetitions pin each pair to fixed nodes and run it in both orders. The geometric mean of the two ratios cancels a multiplicative node effect.

**Validate the method with an A/A test.** Abedi & Brecht validate a methodology by comparing two identical alternatives: a sound method must report no difference. The equivalent here is the same framework on both nodes at the same time, which also measures the node effect directly.

**Consider a duet setup (Bulej et al., ICPE 2020).** Running the two alternatives at the same time on the *same* machine, each on its own CPU, makes interference hit both equally. Comparing their relative performance improved accuracy several-fold in the paper. The assumption that the two do not disturb each other must be tested, for example with an A/A duet. For network-heavy services, the kernel's network processing is shared, so it matters here.

**Reduce noise at the source:**

- **Exclusive cores.** Kubernetes' CPU Manager `static` policy gives Guaranteed pods with integer CPU requests exclusive cores. This removes CFS throttling and sharing with other processes. It differs from Anton Putra's burstable 1500m/2000m setting.
- **SMT off.** GKE's `--threads-per-core=1` disables SMT. Google notes that SMT can "add nondeterministic variance" to compute-bound jobs. It can only be set when a node pool is created, and billing is unchanged.

**Beware measurement bias (Mytkowicz et al., ASPLOS 2009).** Seemingly innocuous setup details, such as the size of the environment variables or link order, can shift results enough to reverse conclusions. Their remedy is setup randomization. Differences of a few percent therefore need more evidence than one configuration can give.

## 7. Use sound statistics

Hoefler & Belli (SC 2015) give twelve rules for reporting results. Those that apply here:

- **Rule 5.** Report whether measurements are deterministic, and give confidence intervals when they are not.
- **Rule 6.** Do not assume normality without checking. With few repetitions, prefer medians, ranges or nonparametric methods.
- **Rule 7.** "Compare nondeterministic data in a statistically sound way", for example with non-overlapping confidence intervals.
- **Rule 3–4.** Summarize costs with the arithmetic mean and rates with the harmonic mean. Avoid summarizing ratios, but if you must, use the geometric mean. The node-free comparison here is a geometric mean of ratios, because the node effect is multiplicative.
- **Rule 8.** Check whether a median or a higher percentile is the right summary. For latency SLOs it is a percentile.

**Decision rule used in this project.** A difference between two frameworks within one tier is claimed only if both node orders of a crossover agree in direction, and the difference is larger than the run-to-run variation of the same framework on the same node. That variation is measured from the repetitions and reported with the results. Otherwise the result is reported as "no significant difference".

Prefer a continuous metric (peak achieved RPS) to a quantized one (the maximum passing 5k step) when testing for differences.

## 8. Report everything needed to reproduce and judge

The SPEC principles P3, P4, P7 and P8, and Hoefler's rules 9, 11 and 12, require:

- **The setup (P3).** Recorded in `gke/README.md` and below.
- **Artifacts (P4).** Code, manifests, raw schedules and analysis scripts are in this repository.
- **Units (P7).** Stated on every column and axis.
- **Cost (P8).** The cost model and resource usage.
- **Upper bounds (Hoefler rule 11).** Dashed lines for the target rate, the SLO and the CPU limit.
- **Only meaningful lines (Hoefler rule 12).** Connect points only where a trend is meaningful.

**Setup of the GKE runs.** Go 1.26.5; gin v1.12.0, chi v5.3.2, echo v5.3.1, fiber v3.5.0, fasthttp v1.74.0; GKE 1.35.8-gke.1036000; Container-Optimized OS with kernel 6.12.94 and containerd 2.2.7. App nodes: n2-standard-4 (Intel Cascade Lake, SMT on, 3.92 allocatable CPUs). Client node: n2-highcpu-16. Zone: us-central1-b.

## 9. Checklist for the next benchmark

1. Write down the question, the metric and the SLO before running anything.
2. Choose open or closed load deliberately, set the arrival distribution and check that concurrency exceeds the bandwidth-delay product.
3. Measure latency from the scheduled time and merge histograms, never percentiles.
4. Prove the load generator has headroom: its CPU, throttling and whether it reaches the target.
5. Validate the numbers against an independent source (kernel counters, the server's own counts) and against physical limits. Validate the metrics pipeline as well.
6. Identify levels of variation (machine, process, run). Repeat at the highest level, randomize the order, block known factors such as the node, and run an A/A test.
7. Reduce noise where possible: exclusive cores, SMT off, fixed placement.
8. Report medians with spread or confidence intervals. Claim a difference only when it is statistically supported.
9. Record the setup, versions, units, cost and limitations, and publish the scripts and data.

## Sources

- A. V. Papadopoulos et al., "Methodological Principles for Reproducible Performance Evaluation in Cloud Computing", IEEE TSE (SPEC Research Group). https://ieeexplore.ieee.org/document/8758926/
- A. Abedi, T. Brecht, "Conducting Repeatable Experiments in Highly Variable Cloud Computing Environments", ICPE 2017. https://cs.uwaterloo.ca/~brecht/papers/icpe-rmit-2017.pdf
- T. Kalibera, R. Jones, "Rigorous Benchmarking in Reasonable Time", ISMM 2013. https://kar.kent.ac.uk/33611/
- T. Hoefler, R. Belli, "Scientific Benchmarking of Parallel Computing Systems: Twelve ways to tell the masses when reporting performance results", SC 2015. https://htor.inf.ethz.ch/publications/img/hoefler-scientific-benchmarking.pdf
- T. Mytkowicz et al., "Producing Wrong Data Without Doing Anything Obviously Wrong!", ASPLOS 2009. https://dl.acm.org/doi/10.1145/1508244.1508275
- L. Bulej et al., "Duet Benchmarking: Improving Measurement Accuracy in the Cloud", ICPE 2020. https://arxiv.org/abs/2001.05811
- M. Kogias et al., "Lancet: A self-correcting Latency Measuring Tool", USENIX ATC 2019. https://www.usenix.org/conference/atc19/presentation/kogias-lancet
- Y. Zhang et al., "Treadmill: Attributing the Source of Tail Latency through Precise Load Testing and Statistical Inference", ISCA 2016. https://ieeexplore.ieee.org/document/7551414/
- B. Schroeder, A. Wierman, M. Harchol-Balter, "Open Versus Closed: A Cautionary Tale", NSDI 2006. https://www.usenix.org/conference/nsdi-06/open-versus-closed-cautionary-tale
- A. Maricq et al., "Taming Performance Variability", OSDI 2018. https://www.usenix.org/conference/osdi18/presentation/maricq
- G. Tene, "How NOT to Measure Latency". https://www.youtube.com/watch?v=lJ8ydIuPFeU
- B. Gregg, "Active Benchmarking" and "Evaluating the Evaluation: A Benchmarking Checklist". https://www.brendangregg.com/activebenchmarking.html, https://www.brendangregg.com/blog/2018-06-30/benchmarking-checklist.html
- Kubernetes, "Control CPU Management Policies on the Node". https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/
- Google Cloud, "Configure simultaneous multi-threading (SMT)" for GKE. https://cloud.google.com/kubernetes-engine/docs/how-to/configure-smt

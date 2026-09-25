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
- **Local experiment.** The stdlib app ran on 1 core, driven by a tester variant that draws exponential gaps. Evenly spaced and Poisson arrivals ran interleaved in random order, with an **A/A control**: a second, identical evenly-spaced configuration. There were 5 rounds of 15 s per trial. Raw data is in `arrival-experiment-results.json` and `arrival-aa-results.json`.
  - **p50 differs clearly.** At 78% load, Poisson arrivals gave a median latency of 0.66–0.77 ms, against 0.19–0.32 ms for both evenly spaced configurations in every round. This is the direction queueing theory predicts.
  - **p99 did not differ beyond noise.** The two *identical* configurations differed as much as evenly spaced vs. Poisson did. For example, at 92% load p99 ranged over 55–222 ms among identical runs. A first 3-round experiment without the A/A control had suggested that evenly spaced arrivals give a worse p99. That was a false positive, which the A/A control exposed.
  - **Near saturation, evenly spaced runs were bimodal.** At 92% load their p50 was either about 0.5 ms or 15–37 ms, while Poisson runs stayed at 1.0–1.6 ms. Single runs near the knee can be far from typical.
- **Conclusion:** the arrival process changes the body of the latency distribution. In this environment its effect on p99 was smaller than run-to-run noise. The GKE results are stated for evenly spaced arrivals.

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
- **SMT off.** GKE's `--threads-per-core=1` disables SMT. Google notes that SMT can "add nondeterministic variance" to compute-bound jobs, and the experiment above confirms it for parallel work within a VM. It can only be set when a node pool is created, and billing is unchanged.

**Measured in this project.** The following experiments were run on separate VMs in other zones, so they could not disturb the GKE benchmark. Code is in `experiments/`, data in `results/vmvar-*` and `results/e2evar-*`.

| Question | Setup | Result |
|---|---|---|
| Do fresh VMs of one type differ in CPU speed? | 5 fresh n2-standard-4 VMs (all Cascade Lake). JSON encoding of the benchmark payload and an integer loop, 10 × 3 s each. | Barely. Coefficient of variation between VMs: 0.48% (JSON) and 0.07% (integer loop). |
| Do they differ in end-to-end HTTP capacity? | 5 fresh n2-standard-2 app VMs running the stdlib app pinned to 1 vCPU, one fixed client VM, and a 5k–45k RPS ramp. | Yes. Peak achieved RPS was 36.1k–41.1k (CV 4.1%), and the maximum rate within the SLO was 30k–40k. |
| Is that the VM or the time of the run? | 3 fresh app VMs, each measured 3 times in a row. | The VM. Within one VM, peak varied by 0.9–2.0%. Between VMs (medians) it varied by 8.2%, and one VM was slowest in all three of its runs. |
| Does the GKE node effect come from network latency? | Low-load (5–15k RPS) latency on the two GKE app nodes, from the benchmark data. | No. p50 differed by 2 µs and p90 by 6 µs. The slower node used about 3% more CPU per request. |
| Does SMT matter? | 3 fresh n2-standard-4 VMs with `--threads-per-core=1`, compared with the 5 SMT-on VMs. | For two parallel workers, yes. With SMT on, 2 vCPUs can be two hyperthreads of one core: JSON encoding with 2 goroutines was about 25% slower per operation (1,376–1,399 vs. 1,031–1,057 ns), and its within-VM spread was larger (2.8–7.1% vs. 2.4–2.7%). Single-thread speed was the same. |

**What this means:**

- **The VM you get changes end-to-end results by several percent.** This happens even though the CPU cores are equally fast, so the variation lies in the I/O and network path rather than in compute.
- **Repeating runs on one VM gives false precision.** Its runs agree within about 1–2% while the absolute number can be about 8% off for another VM. This is exactly Kalibera and Jones' point about repeating at the highest level that varies.
- **For comparisons,** either run every candidate on the same machines (blocking, crossover or duet) or sample many VMs.
- **Absolute numbers** from a few VMs carry an uncertainty of a few percent that the repetitions do not show.
- **"2 vCPUs" on an SMT machine is not 2 physical cores.** A 2-CPU container limit on a 4-vCPU SMT node can land on one core's two hyperthreads.

**Beware measurement bias (Mytkowicz et al., ASPLOS 2009).** Seemingly innocuous setup details, such as the size of the environment variables or link order, can shift results enough to reverse conclusions. Their remedy is setup randomization. Differences of a few percent therefore need more evidence than one configuration can give.

## 7. Use sound statistics

Hoefler & Belli (SC 2015) give twelve rules for reporting results. Those that apply here:

- **Rule 5.** Report whether measurements are deterministic, and give confidence intervals when they are not.
- **Rule 6.** Do not assume normality without checking. With few repetitions, prefer medians, ranges or nonparametric methods.
- **Rule 7.** "Compare nondeterministic data in a statistically sound way", for example with non-overlapping confidence intervals.
- **Rule 3–4.** Summarize costs with the arithmetic mean and rates with the harmonic mean. Avoid summarizing ratios, but if you must, use the geometric mean. The node-free comparison here is a geometric mean of ratios, because the node effect is multiplicative.
- **Rule 8.** Check whether a median or a higher percentile is the right summary. For latency SLOs it is a percentile.

**Decision rule used in this project.** Two frameworks are compared with the node-free ratio of their peak achieved RPS: the geometric mean of the ratios from both node orders of a crossover. A difference is claimed only if this ratio differs from 1 by more than a threshold measured from the data: `2 × run-to-run noise + node-effect spread`.

- **Run-to-run noise** is the median spread (max/min − 1) of the same framework on the same node.
- **Node-effect spread** is half the range of the per-framework node ratios. The node effect is not identical for every framework, so the crossover cancels only its average.
- Anything within the threshold is reported as "no significant difference".

A first version of this rule required both node orders to agree in direction. That is wrong whenever the node effect (about 5%) is larger than the framework difference: the two orders then point in opposite directions by construction, even though their geometric mean removes the node effect.

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
6. Identify levels of variation (machine, process, run). Repeat at the highest level, which in the cloud is the VM: repeats on one VM understate the uncertainty. Randomize the order, block known factors such as the node (every candidate on the same machines), and run an A/A test.
7. Reduce noise where possible: exclusive cores, SMT off, fixed placement.
8. Report medians with spread or confidence intervals. Claim a difference only when it is statistically supported.
9. Record the setup, versions, units, cost and limitations, and publish the scripts and data.

## 10. Comparison with Anton Putra's setup, and a faster design

Anton Putra benchmarks many alternatives in one video. His public repository (`antonputra/tutorials`, lessons 229, 250, 258 and 275) shows how. Only what the manifests show is stated here. His videos could not be checked, because YouTube blocks requests from cloud IP addresses.

**What his manifests show:**

- Apps run on nodes labelled `node=general`, and load generators run on separate nodes labelled `node=clients`.
- The load generator is a Kubernetes Job with 19–40 parallel pods of 1 CPU each.
- The load is closed-loop: the number of clients rises from 1 to 1,000 in 15 s stages, with a 40 ms delay between requests and a 1 s timeout. The request rate is a result of the client count, not a scheduled target.
- Monitoring uses Prometheus with a 5 s scrape interval, a standalone cAdvisor DaemonSet, kube-state-metrics and Grafana.
- In lesson 275, each app has two replicas with pod affinity to itself, so both replicas run on the same node.

**What is unknown:** whether two different apps ever share a node, how many nodes were used, and whether runs were repeated. The repository shows no repetitions, no rotation of apps across nodes and no request accounting.

**Why his benchmarks are fast:** all candidates run at the same time, so one ramp covers every framework. The speed comes from this design, not from special tooling.

**What that design misses:** with one app per node and one run, the framework and the node are confounded. This project measured node effects of about 5% between identical GKE nodes and up to about 8% between fresh VMs, which is larger than most differences within a tier.

**A faster design that keeps the safeguards:**

1. Run all six frameworks at the same time, each on its own app node, with the testers on a shared client node pool.
2. Rotate frameworks across nodes in every repetition (a Latin square). After six repetitions, every framework has run on every node exactly once, so the node effect is balanced and can also be estimated.

   | Repetition | node 1 | node 2 | node 3 | node 4 | node 5 | node 6 |
   |---|---|---|---|---|---|---|
   | 1 | stdlib | chi | gin | echo | fiber | fasthttp |
   | 2 | chi | gin | echo | fiber | fasthttp | stdlib |
   | ... | | | | | | |
   | 6 | fasthttp | stdlib | chi | gin | echo | fiber |

3. Keep the open-model tester, the validation script and the decision rule from section 7. Use Grafana for watching runs, not as the source of final numbers.
4. Never put several frameworks on one node for a throughput comparison. They would compete for CPU, memory bandwidth, cache and the network interface.

**Cost and constraints:** one repetition takes about 20 minutes instead of about 1.5 hours. Six app nodes (n2-standard-4) plus the client node need 40 vCPUs, above this project's 32-vCPU quota. Without a quota increase, three app nodes with two batches per repetition keep the same balance at twice the time.

This design has not been run yet.

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

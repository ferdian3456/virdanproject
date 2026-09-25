#!/usr/bin/env python3
"""Summarize a GKE benchmark run from Google Cloud Managed Service for Prometheus.

Usage: gke/analyze.py results/gke-<RUN_ID> [--slo-ms 50]

Reads runs.csv (written by gke/run.sh), queries GMP once per run (framework,
scenario, repetition), and writes stages.csv (one row per load stage and
repetition), summary.md (per-repetition results and medians) and one chart per
scenario into the run directory.

Definitions, per stage:
- scheduled: target rate x window length (what the users sent);
- achieved: 2xx responses per second;
- errors: non-2xx responses, timeouts and dropped requests (never sent because
  their deadline passed while every connection was busy) per second;
- availability: 2xx responses within the deadline divided by scheduled requests;
- SLO pass: p99 <= --slo-ms, achieved >= 95% of target, errors <= 1% of target.
"""
import argparse
import csv
import json
import math
import statistics
import subprocess
from pathlib import Path

PROMQL = Path(__file__).with_name("promql.sh")
SKIP_S = 5  # drop the first seconds of each stage, while the rate is still changing
SCRAPE_S = 5  # PodMonitoring interval for the testers (gke/k8s/base.yaml)
# App CPU window: the stage plus this many stages on each side. cAdvisor timestamps lag
# by ~10-15 s, so shorter windows read above the 2-core quota (see results/*/validation.md).
CPU_PAD_STAGES = 2
COLORS = {"stdlib": "#2a78d6", "chi": "#eb6834", "gin": "#1baf7a",
          "echo": "#eda100", "fiber": "#e87ba4", "fasthttp": "#008300"}
MARKERS = {"stdlib": "o", "chi": "s", "gin": "^", "echo": "D", "fiber": "v", "fasthttp": "P"}


def promql(*args):
    out = subprocess.run([str(PROMQL), *map(str, args)], check=True, capture_output=True, text=True).stdout
    data = json.loads(out)
    if data["status"] != "success":
        raise RuntimeError(f"{args[0]}: {data}")
    return data["data"]["result"]


def by_label(query, label, start, end, step):
    """Range query; returns {label value: {timestamp: value}}."""
    return {r["metric"].get(label, ""): {int(float(t)): float(v) for t, v in r["values"]}
            for r in promql(query, start, end, step)}


def raw_samples(selector, end, span):
    """Raw samples of a single series in (end - span, end]."""
    result = promql(f"{selector}[{span}s] @ {end}")
    if len(result) > 1:
        raise RuntimeError(f"{selector}: expected one series, got {len(result)}")
    return [(float(t), float(v)) for t, v in (result[0]["values"] if result else [])]


def quantile(q, buckets):
    """Prometheus histogram_quantile over {upper bound: cumulative count}."""
    bounds = sorted(buckets)
    total = buckets[bounds[-1]]
    if total <= 0:
        return None
    rank, prev_b, prev_c = q * total, 0.0, 0.0
    for b in bounds:
        if buckets[b] >= rank:
            if math.isinf(b):
                return prev_b
            return prev_b + (b - prev_b) * (rank - prev_c) / (buckets[b] - prev_c)
        prev_b, prev_c = b, buckets[b]
    return None


def counter_at(samples, t):
    """Counter value at t, linearly interpolated between the raw samples around it.

    Interpolation estimated saturated app CPU at 1.97 +/- 0.06 cores against the
    2-core quota; slopes between first/last distinct samples read 2.04 +/- 0.08.
    """
    before = [p for p in samples if p[0] <= t]
    after = [p for p in samples if p[0] >= t]
    if not before or not after:
        return None
    (ta, va), (tb, vb) = before[-1], after[0]
    return va if tb == ta else va + (vb - va) * (t - ta) / (tb - ta)


def increase(samples, t0, t1):
    a, b = counter_at(samples, t0), counter_at(samples, t1)
    return None if a is None or b is None else b - a


def stage_metrics(run):
    """Per stage, uses the window [stage start + SKIP_S, stage end].

    Tester counters are summed across pods every SCRAPE_S seconds and differenced
    between the window edges, so no rate() extrapolation is involved: each pod's
    latest sample sits at the same scrape phase at both edges. cAdvisor refreshes
    irregularly and its timestamps lag, so app CPU uses a window centred on the
    stage, widened by CPU_PAD_STAGES stages on each side; on a linear ramp its
    average equals the stage's own value. Throttling uses the stage itself.
    """
    fw, scen, stage_s = run["framework"], run["scenario"], int(run["stage_s"])
    start, stages = int(run["start_at"]), int(run["stages"])
    end = start + stages * stage_s
    if "rep" in run:
        # The run label keeps out earlier runs' tester series, which GMP still returns
        # (flat) for up to 5 minutes after those pods are gone.
        t = f'namespace="bench",run="{fw}-{scen}-r{run["rep"]}"'
    else:  # runs before repetitions were introduced
        t = f'namespace="bench",framework="{fw}",method="{scen}"'
    counts = by_label(f"sum by (status) (tester_request_duration_seconds_count{{{t}}})", "status", start, end, SCRAPE_S)
    if not counts:
        raise RuntimeError(f"no tester data for {fw} {scen} rep {run.get('rep')}")
    buckets = by_label(f"sum by (le) (tester_request_duration_seconds_bucket{{{t}}})", "le", start, end, SCRAPE_S)
    buckets = {float(le): v for le, v in buckets.items()}
    deadline_s = float(run.get("deadline_ms") or 0) / 1000
    on_time = dropped = {}
    if deadline_s:
        le = f"{deadline_s:g}"
        on_time = by_label(f'sum(tester_request_duration_seconds_bucket{{{t},status=~"2..",le="{le}"}})', "", start, end, SCRAPE_S).get("", {})
        dropped = by_label(f"sum(tester_dropped_requests_total{{{t}}})", "", start, end, SCRAPE_S).get("", {})
        if not on_time or not dropped:
            raise RuntimeError(f"missing on-time bucket le={le} or dropped counter for {t}")
    app = f'namespace="bench",container="app",pod=~"{fw}-[a-z0-9]{{6,10}}-[a-z0-9]{{5}}"'
    span = end - start + 2 * stage_s  # samples just outside [start, end] for interpolation
    cpu = raw_samples(f"container_cpu_usage_seconds_total{{{app}}}", end + stage_s, span)
    thr = raw_samples(f"container_cpu_cfs_throttled_periods_total{{{app}}}", end + stage_s, span)
    per = raw_samples(f"container_cpu_cfs_periods_total{{{app}}}", end + stage_s, span)

    def delta(series, t0, t1):
        # A series first seen after t0 (e.g. the first timeout) started from zero.
        if t1 not in series:
            return None
        if t0 in series:
            return series[t1] - series[t0]
        return series[t1] if min(series) > t0 else None

    rows = []
    for k in range(stages):
        t0, t1 = start + k * stage_s + SKIP_S, start + (k + 1) * stage_s
        dt = t1 - t0
        target = int(run["pods"]) * (float(run["start_rps_pod"]) + k * float(run["step_rps_pod"]))
        row = {"scenario": scen, "framework": fw, "rep": int(run.get("rep", 1)), "pair": run["pair"],
               "stage": k, "target_rps": round(target)}
        ok = sum(d for s, v in counts.items() if s.startswith("2") and (d := delta(v, t0, t1)) is not None)
        bad = sum(d for s, v in counts.items() if not s.startswith("2") and (d := delta(v, t0, t1)) is not None)
        drop = delta(dropped, t0, t1) if dropped else 0.0
        if drop is None:
            raise RuntimeError(f"dropped counter has no samples at {t0} or {t1} for {t}")
        row["rps"], row["errors_rps"], row["dropped_rps"] = ok / dt, (bad + drop) / dt, drop / dt
        good = delta(on_time, t0, t1) if on_time else None
        row["availability"] = None if good is None else good / (target * dt)
        hist = {le: d for le, v in buckets.items() if (d := delta(v, t0, t1)) is not None}
        for name, q in [("p50", 0.5), ("p90", 0.9), ("p99", 0.99), ("p999", 0.999)]:
            v = quantile(q, hist) if hist else None
            row[name] = None if v is None else v * 1000  # seconds -> ms
        s0 = max(start, start + (k - CPU_PAD_STAGES) * stage_s)
        s1 = min(end, start + (k + 1 + CPU_PAD_STAGES) * stage_s)
        d_cpu = increase(cpu, s0, s1)
        row["cpu_cores"] = None if d_cpu is None else d_cpu / (s1 - s0)
        # Throttling is a ratio of two counters refreshed together, so the timestamp lag
        # cancels out and the stage's own window keeps the knee sharp.
        c0, c1 = start + k * stage_s, start + (k + 1) * stage_s
        d_thr, d_per = increase(thr, c0, c1), increase(per, c0, c1)
        row["throttled"] = d_thr / d_per if d_thr is not None and d_per else None
        rows.append(row)
    return rows


def passed(r, slo_ms):
    return (r["p99"] is not None and r["p99"] <= slo_ms and r["rps"] >= 0.95 * r["target_rps"]
            and r["errors_rps"] <= 0.01 * r["target_rps"])


def median(values):
    values = [v for v in values if v is not None]
    return statistics.median(values) if values else None


def fmt(v, spec=".2f"):
    return "-" if v is None else format(v, spec)


def chart(rows, scenario, slo_ms, path):
    """Median across repetitions per stage; the band spans the lowest and highest repetition."""
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    panels = [("rps", "Achieved RPS (2xx)", 1 / 1000, "k RPS"),
              ("p99", f"p99 latency (SLO {slo_ms:g} ms)", 1, "ms, log scale"),
              ("availability", "Availability (2xx within deadline / scheduled)", 100, "%"),
              ("cpu_cores", "App CPU usage", 1, "cores (limit 2)"),
              ("throttled", "App CPU throttling", 100, "% of CFS periods")]
    fig, axes = plt.subplots(2, 3, figsize=(18, 9), facecolor="#fcfcfb")
    frameworks = [f for f in COLORS if any(r["framework"] == f for r in rows)]
    targets = sorted({r["target_rps"] for r in rows})
    reps = sorted({r["rep"] for r in rows})
    for ax, (key, title, scale, unit) in zip(axes.flat, panels):
        ax.set_facecolor("#fcfcfb")
        for fw in frameworks:
            x, mid, lo, hi = [], [], [], []
            for tg in targets:
                vals = [r[key] * scale for r in rows if r["framework"] == fw and r["target_rps"] == tg and r[key] is not None]
                if vals:
                    x.append(tg / 1000); mid.append(statistics.median(vals)); lo.append(min(vals)); hi.append(max(vals))
            if not x:
                continue
            ax.fill_between(x, lo, hi, color=COLORS[fw], alpha=0.15, lw=0)
            ax.plot(x, mid, color=COLORS[fw], lw=2, marker=MARKERS[fw], ms=4, label=fw)
            ax.annotate(fw, (x[-1], mid[-1]), xytext=(4, 0), textcoords="offset points",
                        fontsize=8, color="#52514e", va="center")
        if key == "rps":
            ax.plot([0, targets[-1] / 1000], [0, targets[-1] / 1000], color="#8f8e88", lw=1, ls="--", label="target")
        if key == "p99":
            ax.set_yscale("log")
            ax.axhline(slo_ms, color="#8f8e88", lw=1, ls="--")
        if key == "cpu_cores":
            ax.axhline(2, color="#8f8e88", lw=1, ls="--")
        if key == "availability":
            ax.set_ylim(-5, 105)
        ax.set_title(title, loc="left", fontsize=11, color="#0b0b0b")
        ax.set_xlabel("Target rate (k RPS)", fontsize=9, color="#52514e")
        ax.set_ylabel(unit, fontsize=9, color="#52514e")
        ax.grid(color="#e6e5e0", lw=0.8)
        ax.tick_params(colors="#52514e", labelsize=8)
        for side in ("top", "right"):
            ax.spines[side].set_visible(False)
        for side in ("left", "bottom"):
            ax.spines[side].set_color("#c3c2b7")
    note = axes.flat[5]
    note.axis("off")
    note.text(0, 0.9, "\n".join([
        f"Lines: median of {len(reps)} repetition(s) per stage.",
        "Bands: lowest to highest repetition.",
        "Dashed: target rate, p99 SLO, 2-core CPU limit.",
        "Latency is measured from each request's scheduled",
        "send time (no coordinated omission).",
        "CPU: 150 s window centred on the stage (about",
        "+/-0.1 core); it smooths the knee.",
    ]), fontsize=9, color="#52514e", va="top", family="monospace")
    handles, labels = axes.flat[0].get_legend_handles_labels()
    fig.suptitle(f"{scenario.upper()} /api/devices on GKE (2 CPU limit, open-model load)", y=0.99, fontsize=13)
    fig.legend(handles, labels, loc="upper center", bbox_to_anchor=(0.5, 0.955), ncol=len(labels), frameon=False, fontsize=9)
    fig.tight_layout(rect=(0, 0, 1, 0.92))
    fig.savefig(path, dpi=130)
    plt.close(fig)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("run_dir", type=Path)
    ap.add_argument("--slo-ms", type=float, default=50.0)
    args = ap.parse_args()

    with (args.run_dir / "runs.csv").open() as fh:
        runs = list(csv.DictReader(fh))
    rows = [row for run in runs for row in stage_metrics(run)]
    for r in rows:
        r["passed"] = passed(r, args.slo_ms)

    fields = ["scenario", "framework", "rep", "pair", "stage", "target_rps", "rps", "errors_rps", "dropped_rps",
              "availability", "p50", "p90", "p99", "p999", "cpu_cores", "throttled", "passed"]
    with (args.run_dir / "stages.csv").open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=fields)
        w.writeheader()
        w.writerows(rows)

    reps = sorted({r["rep"] for r in rows})
    out = [f"# GKE benchmark summary: {args.run_dir.name}", "",
           f"SLO: p99 <= {args.slo_ms:g} ms, achieved >= 95% of target rate, errors (non-2xx, timeouts and dropped "
           f"requests) <= 1% of target. {len(reps)} repetition(s); tables show per-repetition values and medians. "
           f"Tester metrics cover each 30 s stage minus its first {SKIP_S} s. App CPU comes from cAdvisor over a "
           f"{(2 * CPU_PAD_STAGES + 1) * 30} s window centred on each stage, because cAdvisor timestamps lag by "
           "~10-15 s (accuracy about +/-0.1 core; it smooths the knee). Throttling uses the stage itself.", ""]
    for scenario in ("get", "post"):
        srows = [r for r in rows if r["scenario"] == scenario]
        if not srows:
            continue
        frameworks = sorted({r["framework"] for r in srows})
        best = {(fw, rep): max((r["target_rps"] for r in srows if r["framework"] == fw and r["rep"] == rep and r["passed"]), default=0)
                for fw in frameworks for rep in reps}
        peak = {(fw, rep): max((r["rps"] for r in srows if r["framework"] == fw and r["rep"] == rep), default=0)
                for fw in frameworks for rep in reps}
        med_best = {fw: median(best[fw, rep] for rep in reps) for fw in frameworks}
        med_peak = {fw: median(peak[fw, rep] for rep in reps) for fw in frameworks}
        rep_cols = " | ".join(f"Rep {rep}" for rep in reps)
        out += [f"## {scenario.upper()} /api/devices", "", f"![{scenario} chart]({scenario}.png)", "",
                "### Max rate within SLO (RPS)", "",
                f"| Framework | {rep_cols} | Median | Median peak achieved RPS |",
                "|---|" + "---|" * len(reps) + "---|---|"]
        for fw in sorted(frameworks, key=lambda f: (-med_best[f], -med_peak[f])):
            cells = " | ".join(f"{best[fw, rep]:,}" for rep in reps)
            out.append(f"| {fw} | {cells} | {med_best[fw]:,.0f} | {med_peak[fw]:,.0f} |")
        out += ["", "### Per stage (median across repetitions)", "",
                "| Target RPS | Framework | Achieved RPS | Errors/s | Availability | p50 ms | p90 ms | p99 ms | p99.9 ms | CPU cores | Throttled | SLO passed |",
                "|---|---|---|---|---|---|---|---|---|---|---|---|"]
        for target in sorted({r["target_rps"] for r in srows}):
            for fw in frameworks:
                g = [r for r in srows if r["framework"] == fw and r["target_rps"] == target]
                if not g:
                    continue
                m = {k: median(r[k] for r in g) for k in ("rps", "errors_rps", "availability", "p50", "p90", "p99", "p999", "cpu_cores", "throttled")}
                out.append(f"| {target:,} | {fw} | {fmt(m['rps'], ',.0f')} | {fmt(m['errors_rps'], ',.0f')} | "
                           f"{fmt(None if m['availability'] is None else m['availability'] * 100, '.1f')}% | "
                           f"{fmt(m['p50'])} | {fmt(m['p90'])} | {fmt(m['p99'])} | {fmt(m['p999'])} | "
                           f"{fmt(m['cpu_cores'])} | {fmt(None if m['throttled'] is None else m['throttled'] * 100, '.1f')}% | "
                           f"{sum(r['passed'] for r in g)}/{len(g)} |")
        out.append("")
        chart(srows, scenario, args.slo_ms, args.run_dir / f"{scenario}.png")

    (args.run_dir / "summary.md").write_text("\n".join(out))
    print("\n".join(out[:40]))


if __name__ == "__main__":
    main()

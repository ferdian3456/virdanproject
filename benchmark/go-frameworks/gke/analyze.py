#!/usr/bin/env python3
"""Summarize a GKE benchmark run from Google Cloud Managed Service for Prometheus.

Usage: gke/analyze.py results/gke-<RUN_ID> [--slo-ms 50]

Reads runs.csv (written by gke/run.sh), queries GMP once per framework and
scenario, and writes stages.csv (one row per load stage), summary.md and one
chart per scenario into the run directory.
"""
import argparse
import csv
import json
import math
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
    fw, stage_s = run["framework"], int(run["stage_s"])
    start, stages = int(run["start_at"]), int(run["stages"])
    end = start + stages * stage_s
    # method= keeps out the previous scenario's tester series, which GMP still returns
    # (flat) for up to 5 minutes after those pods are gone.
    t = f'namespace="bench",framework="{fw}",method="{run["scenario"]}"'
    counts = by_label(f"sum by (status) (tester_request_duration_seconds_count{{{t}}})", "status", start, end, SCRAPE_S)
    if not counts:
        raise RuntimeError(f"no tester data for {fw} {run['scenario']}")
    buckets = by_label(f"sum by (le) (tester_request_duration_seconds_bucket{{{t}}})", "le", start, end, SCRAPE_S)
    buckets = {float(le): v for le, v in buckets.items()}
    app = f'namespace="bench",container="app",pod=~"{fw}-[a-z0-9]+-[a-z0-9]+"'
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
        target = int(run["pods"]) * (float(run["start_rps_pod"]) + k * float(run["step_rps_pod"]))
        row = {"scenario": run["scenario"], "framework": fw, "pair": run["pair"], "stage": k, "target_rps": round(target)}
        ok = sum(d for s, v in counts.items() if s.startswith("2") and (d := delta(v, t0, t1)) is not None)
        bad = sum(d for s, v in counts.items() if not s.startswith("2") and (d := delta(v, t0, t1)) is not None)
        row["rps"], row["errors_rps"] = ok / (t1 - t0), bad / (t1 - t0)
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
    rps, err = r["rps"] or 0.0, r["errors_rps"] or 0.0
    return (r["p99"] is not None and r["p99"] <= slo_ms and rps >= 0.95 * r["target_rps"]
            and err <= 0.01 * (rps + err))


def fmt(v, spec=".2f"):
    return "-" if v is None else format(v, spec)


def chart(rows, scenario, path):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    panels = [("rps", "Achieved RPS (2xx)", 1 / 1000, "k RPS"), ("p99", "p99 latency", 1, "ms, log scale"),
              ("cpu_cores", "App CPU usage", 1, "cores (limit 2)"), ("throttled", "App CPU throttling", 100, "% of CFS periods")]
    fig, axes = plt.subplots(2, 2, figsize=(13, 8.5), facecolor="#fcfcfb")
    frameworks = [f for f in COLORS if any(r["framework"] == f for r in rows)]
    for ax, (key, title, scale, unit) in zip(axes.flat, panels):
        ax.set_facecolor("#fcfcfb")
        for fw in frameworks:
            pts = [(r["target_rps"] / 1000, r[key] * scale) for r in rows if r["framework"] == fw and r[key] is not None]
            if not pts:
                continue
            x, y = zip(*pts)
            ax.plot(x, y, color=COLORS[fw], lw=2, marker=MARKERS[fw], ms=4, label=fw)
            ax.annotate(fw, (x[-1], y[-1]), xytext=(4, 0), textcoords="offset points",
                        fontsize=8, color="#52514e", va="center")
        if key == "rps":
            lim = max(r["target_rps"] for r in rows) / 1000
            ax.plot([0, lim], [0, lim], color="#8f8e88", lw=1, ls="--", label="target")
        if key == "p99":
            ax.set_yscale("log")
        ax.set_title(title, loc="left", fontsize=11, color="#0b0b0b")
        ax.set_xlabel("Target rate (k RPS)", fontsize=9, color="#52514e")
        ax.set_ylabel(unit, fontsize=9, color="#52514e")
        ax.grid(color="#e6e5e0", lw=0.8)
        ax.tick_params(colors="#52514e", labelsize=8)
        for side in ("top", "right"):
            ax.spines[side].set_visible(False)
        for side in ("left", "bottom"):
            ax.spines[side].set_color("#c3c2b7")
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

    fields = ["scenario", "framework", "pair", "stage", "target_rps", "rps", "errors_rps",
              "p50", "p90", "p99", "p999", "cpu_cores", "throttled", "passed"]
    with (args.run_dir / "stages.csv").open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=fields)
        w.writeheader()
        w.writerows(rows)

    out = [f"# GKE benchmark summary: {args.run_dir.name}", "",
           f"SLO: p99 <= {args.slo_ms:g} ms, achieved >= 95% of target rate, errors <= 1%. "
           f"Tester metrics cover each 30 s stage minus its first {SKIP_S} s. "
           f"App CPU comes from cAdvisor over a {(2 * CPU_PAD_STAGES + 1) * 30} s window centred on each stage, "
           "because cAdvisor timestamps lag by ~10-15 s (accuracy about +/-0.1 core; it smooths the knee). "
           "Throttling uses the stage itself. See validation.md.", ""]
    for scenario in ("get", "post"):
        srows = [r for r in rows if r["scenario"] == scenario]
        if not srows:
            continue
        frameworks = sorted({r["framework"] for r in srows})
        best = {fw: max((r["target_rps"] for r in srows if r["framework"] == fw and r["passed"]), default=0)
                for fw in frameworks}
        peak = {fw: max((r["rps"] or 0) for r in srows if r["framework"] == fw) for fw in frameworks}
        out += [f"## {scenario.upper()} /api/devices", "", f"![{scenario} chart]({scenario}.png)", "",
                "| Framework | Pair | Max rate within SLO (RPS) | Peak achieved RPS |", "|---|---|---|---|"]
        for fw in sorted(frameworks, key=lambda f: (-best[f], -peak[f])):
            pair = next(r["pair"] for r in srows if r["framework"] == fw)
            out.append(f"| {fw} | {pair} | {best[fw]:,} | {peak[fw]:,.0f} |")
        out += ["", "| Target RPS | Framework | Achieved RPS | Errors/s | p50 ms | p90 ms | p99 ms | p99.9 ms | CPU cores | Throttled | SLO |",
                "|---|---|---|---|---|---|---|---|---|---|---|"]
        for target in sorted({r["target_rps"] for r in srows}):
            for r in (r for r in srows if r["target_rps"] == target):
                out.append(f"| {target:,} | {r['framework']} | {fmt(r['rps'], ',.0f')} | {fmt(r['errors_rps'], ',.0f')} | "
                           f"{fmt(r['p50'])} | {fmt(r['p90'])} | {fmt(r['p99'])} | {fmt(r['p999'])} | "
                           f"{fmt(r['cpu_cores'])} | {fmt(None if r['throttled'] is None else r['throttled'] * 100, '.1f')}% | "
                           f"{'pass' if r['passed'] else 'fail'} |")
        out.append("")
        chart(srows, scenario, args.run_dir / f"{scenario}.png")

    (args.run_dir / "summary.md").write_text("\n".join(out))
    print("\n".join(out[:40]))


if __name__ == "__main__":
    main()

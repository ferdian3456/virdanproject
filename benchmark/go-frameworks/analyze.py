#!/usr/bin/env python3
"""Summarize a benchmark run downloaded by gcp/wait.sh.

Usage: ./analyze.py results/<RUN_ID> [--slo-ms 50]

Writes summary.csv (one row per wrk2 step) and summary.md (medians across
repetitions) into the run directory.
"""
import argparse
import csv
import re
import statistics
from collections import defaultdict
from pathlib import Path

PERCENTILES = {"50.000%": "p50", "90.000%": "p90", "99.000%": "p99", "99.900%": "p999"}
UNITS_MS = {"us": 0.001, "ms": 1.0, "s": 1000.0, "m": 60000.0}
CALIBRATION_S = 10  # wrk2 does not record latency during its first ~10s


def to_ms(value):
    m = re.fullmatch(r"([\d.]+)(us|ms|s|m)", value)
    return float(m.group(1)) * UNITS_MS[m.group(2)]


def parse_wrk2(path):
    r = {"p50": None, "p90": None, "p99": None, "p999": None, "rps": 0.0, "total": 0, "errors": 0}
    for line in path.read_text().splitlines():
        f = line.split()
        if not f:
            continue
        if f[0] in PERCENTILES and len(f) == 2:
            r[PERCENTILES[f[0]]] = to_ms(f[1])
        elif f[0] == "Requests/sec:":
            r["rps"] = float(f[1])
        elif "requests in" in line:
            r["total"] = int(f[0])
        elif line.strip().startswith("Non-2xx"):
            r["errors"] += int(f[-1])
        elif line.strip().startswith("Socket errors:"):
            r["errors"] += sum(int(x.rstrip(",")) for x in f[3::2])
    return r


def load_cpu(run_dir):
    """Returns ({framework: [(epoch, ticks)]}, clk_tck) or ({}, None) if missing."""
    cpu_file, info_file = run_dir / "sut" / "cpu.csv", run_dir / "sut" / "info.txt"
    if not cpu_file.exists() or not info_file.exists():
        return {}, None
    info = dict(line.split("=", 1) for line in info_file.read_text().split())
    samples = defaultdict(list)
    with cpu_file.open() as fh:
        for row in csv.DictReader(fh):
            samples[row["framework"]].append((int(row["epoch"]), int(row["cpu_ticks"])))
    return samples, int(info["clk_tck"])


def cpu_cores(samples, clk_tck, start, end):
    """Average CPU cores used between start and end, from cumulative tick samples."""
    window = [s for s in samples if start <= s[0] <= end]
    if len(window) < 2 or window[-1][0] == window[0][0]:
        return None
    (t0, c0), (t1, c1) = window[0], window[-1]
    return (c1 - c0) / clk_tck / (t1 - t0)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("run_dir", type=Path)
    ap.add_argument("--slo-ms", type=float, default=50.0)
    args = ap.parse_args()
    results = args.run_dir / "results"

    cpu, clk_tck = load_cpu(args.run_dir)
    rows = []
    with (results / "runs.csv").open() as fh:
        for run in csv.DictReader(fh):
            r = parse_wrk2(results / run["scenario"] / run["framework"] / f"r{run['rep']}" / f"{run['rate']}.txt")
            rate = int(run["rate"])
            r["passed"] = (
                r["p99"] is not None
                and r["p99"] <= args.slo_ms
                and r["rps"] >= 0.95 * rate
                and (r["total"] == 0 or r["errors"] / r["total"] <= 0.01)
            )
            r["cpu_cores"] = None
            if clk_tck:
                start = int(run["start_epoch"]) + CALIBRATION_S
                r["cpu_cores"] = cpu_cores(cpu[run["framework"]], clk_tck, start, int(run["end_epoch"]))
            rows.append({"rep": int(run["rep"]), "scenario": run["scenario"], "framework": run["framework"], "rate": rate, **r})

    fields = ["scenario", "framework", "rep", "rate", "rps", "p50", "p90", "p99", "p999", "errors", "total", "cpu_cores", "passed"]
    with (args.run_dir / "summary.csv").open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=fields, extrasaction="ignore")
        w.writeheader()
        w.writerows(rows)

    def med(values):
        values = [v for v in values if v is not None]
        return statistics.median(values) if values else None

    def fmt(v, spec=".2f"):
        return "-" if v is None else format(v, spec)

    out = [f"# Benchmark summary: {args.run_dir.name}", "", f"SLO: p99 <= {args.slo_ms:g} ms, achieved >= 95% of target rate, errors <= 1%.", ""]
    for scenario in sorted({r["scenario"] for r in rows}):
        srows = [r for r in rows if r["scenario"] == scenario]
        frameworks = sorted({r["framework"] for r in srows})

        out += [f"## {scenario.upper()} /api/devices", "", "### Max rate within SLO (median across repetitions)", "",
                "| Framework | Max rate (RPS) |", "|---|---|"]
        best = {}
        for fw in frameworks:
            per_rep = defaultdict(int)
            for r in srows:
                if r["framework"] == fw and r["passed"]:
                    per_rep[r["rep"]] = max(per_rep[r["rep"]], r["rate"])
            reps = {r["rep"] for r in srows if r["framework"] == fw}
            best[fw] = statistics.median(per_rep.get(rep, 0) for rep in reps)
        for fw in sorted(frameworks, key=lambda f: -best[f]):
            out.append(f"| {fw} | {best[fw]:,.0f} |")

        out += ["", "### Latency and CPU per step (median across repetitions)", "",
                "| Rate | Framework | Achieved RPS | p50 ms | p90 ms | p99 ms | p99.9 ms | CPU cores | Passed reps |",
                "|---|---|---|---|---|---|---|---|---|"]
        for rate in sorted({r["rate"] for r in srows}):
            for fw in frameworks:
                g = [r for r in srows if r["framework"] == fw and r["rate"] == rate]
                if not g:
                    continue
                out.append(
                    f"| {rate:,} | {fw} | {fmt(med(r['rps'] for r in g), ',.0f')} | {fmt(med(r['p50'] for r in g))} | "
                    f"{fmt(med(r['p90'] for r in g))} | {fmt(med(r['p99'] for r in g))} | {fmt(med(r['p999'] for r in g))} | "
                    f"{fmt(med(r['cpu_cores'] for r in g))} | {sum(r['passed'] for r in g)}/{len(g)} |"
                )
        out.append("")

    (args.run_dir / "summary.md").write_text("\n".join(out))
    print("\n".join(out))


if __name__ == "__main__":
    main()

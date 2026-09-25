#!/usr/bin/env python3
"""Per-stage results and VM-to-VM spread from an e2evar run.

Usage: experiments/e2evar/analyze.py results/e2evar-<run id>

Each app VM directory holds /metrics snapshots taken at every stage boundary
(snap_0 .. snap_9). Stage k is the difference between snap_k and snap_k+1.
The SLO matches the GKE benchmark: p99 <= 50 ms, achieved >= 95% of the
target, errors (non-2xx, timeouts, dropped) <= 1% of the target.
"""
import json
import math
import re
import statistics
import sys
from pathlib import Path

STAGE_S, START, STEP = 15, 5000, 5000


def parse(path):
    text = path.read_text()
    buckets, counts = {}, {}
    for status, le, v in re.findall(r'tester_request_duration_seconds_bucket\{method="get",status="([^"]+)",le="([^"]+)"\} (\S+)', text):
        if status.startswith("2"):
            buckets[float(le)] = buckets.get(float(le), 0) + float(v)
    for status, v in re.findall(r'tester_request_duration_seconds_count\{method="get",status="([^"]+)"\} (\S+)', text):
        counts[status] = float(v)
    dropped = float(re.search(r"^tester_dropped_requests_total\S* (\S+)", text, re.M).group(1))
    return buckets, counts, dropped


def quantile(q, b):
    bounds = sorted(b)
    total = b[bounds[-1]]
    if total <= 0:
        return None
    rank, pb, pc = q * total, 0.0, 0.0
    for k in bounds:
        if b[k] >= rank:
            return pb if math.isinf(k) else pb + (k - pb) * (rank - pc) / (b[k] - pc)
        pb, pc = k, b[k]


def main():
    run = Path(sys.argv[1])
    summary = {}
    for vm in sorted(p for p in run.iterdir() if p.is_dir()):
        meta = json.loads((vm / "meta.json").read_text())
        snaps = [parse(vm / f"snap_{k}.txt") for k in range(10)]
        print(f"== {vm.name} ({meta['cpu_platform']})")
        best, peak = 0, 0.0
        for k in range(9):
            (b0, c0, d0), (b1, c1, d1) = snaps[k], snaps[k + 1]
            target = START + k * STEP
            ok = sum(v - c0.get(s, 0) for s, v in c1.items() if s.startswith("2")) / STAGE_S
            err = (sum(v - c0.get(s, 0) for s, v in c1.items() if not s.startswith("2")) + d1 - d0) / STAGE_S
            hist = {le: b1[le] - b0.get(le, 0) for le in b1}
            p50, p99 = quantile(0.5, hist), quantile(0.99, hist)
            passed = p99 is not None and p99 <= 0.05 and ok >= 0.95 * target and err <= 0.01 * target
            best = target if passed else best
            peak = max(peak, ok)
            print(f"  {target:6,} | achieved {ok:9,.0f} | errors {err:8,.0f} | p50 {p50 * 1000:8.2f} ms | "
                  f"p99 {p99 * 1000:8.2f} ms | {'pass' if passed else 'fail'}")
        summary[vm.name] = (best, peak)
    peaks = [p for _, p in summary.values()]
    print("\nrun                     max rate within SLO   peak achieved")
    for name, (best, peak) in summary.items():
        print(f"{name:22} {best:12,} {peak:16,.0f}")
    print(f"peak achieved over all runs: spread {(max(peaks) / min(peaks) - 1) * 100:.2f}%, "
          f"coefficient of variation {statistics.pstdev(peaks) / statistics.mean(peaks) * 100:.2f}% (n={len(peaks)})")
    # Several runs per VM (directories <vm>-r<n>): split within-VM and between-VM variation.
    by_vm = {}
    for name, (_, peak) in summary.items():
        by_vm.setdefault(re.sub(r"-r\d+$", "", name), []).append(peak)
    if any(len(v) > 1 for v in by_vm.values()):
        print("\nVM                 runs  median peak   within-VM spread")
        for vm, v in by_vm.items():
            print(f"{vm:18} {len(v):4} {statistics.median(v):12,.0f} {(max(v) / min(v) - 1) * 100:12.2f}%")
        med = [statistics.median(v) for v in by_vm.values()]
        within = [max(v) / min(v) - 1 for v in by_vm.values() if len(v) > 1]
        print(f"between VMs (medians): spread {(max(med) / min(med) - 1) * 100:.2f}%; "
              f"median within-VM spread {statistics.median(within) * 100:.2f}%")


if __name__ == "__main__":
    main()

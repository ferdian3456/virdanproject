#!/usr/bin/env python3
"""Cross-checks the tester's numbers against independent sources.

Usage: gke/validate.py results/gke-<RUN_ID>

For every run in runs.csv and for two halves of the ramp (stages 0-9 and 10-19):
- Requests: the tester's completed requests vs. packets on the app pod's eth0
  (cAdvisor, kernel counters). Each GET/POST request and response fits in one
  packet, so packets per request should be ~1.0 and bytes per packet constant.
  cAdvisor sample timestamps lag by ~10-15 s, which undercounts packets while
  the rate is rising; on a flat (saturated) rate the ratio must be ~1.0.
- App CPU from cAdvisor (what the dashboard uses) vs. GKE system metrics
  (kubernetes_io:container_cpu_core_usage_time); both must stay <= the 2-core limit.
- Tester CPU (busiest pod), to rule out the load generator as the bottleneck.
"""
import csv
import json
import subprocess
import sys
from pathlib import Path

PROMQL = Path(__file__).with_name("promql.sh")


def promql(*args):
    out = subprocess.run([str(PROMQL), *map(str, args)], check=True, capture_output=True, text=True).stdout
    data = json.loads(out)
    if data["status"] != "success":
        raise RuntimeError(f"{args[0]}: {data}")
    return data["data"]["result"]


def raw(selector, t_end, span):
    """{series labels: [(t, v)]} of raw samples in (t_end - span, t_end]."""
    return {json.dumps(r["metric"], sort_keys=True): [(float(t), float(v)) for t, v in r["values"]]
            for r in promql(f"{selector}[{span}s] @ {t_end}")}


def at(samples, t, born_at_zero=False):
    """Counter value at t, linearly interpolated between the surrounding samples.

    born_at_zero: a series first seen after t (e.g. the tester's 2xx counter,
    created by the first request) counted zero at t.
    """
    before = [s for s in samples if s[0] <= t]
    after = [s for s in samples if s[0] >= t]
    if not before and after and born_at_zero:
        return 0.0
    if not before or not after:
        return None
    (t0, v0), (t1, v1) = before[-1], after[0]
    return v0 if t1 == t0 else v0 + (v1 - v0) * (t - t0) / (t1 - t0)


def total_delta(series, t0, t1, born_at_zero=False):
    """Sum over series of the counter increase between t0 and t1 (None if no series or any lacks coverage)."""
    if not series:
        return None
    total = 0.0
    for samples in series.values():
        a, b = at(samples, t0, born_at_zero), at(samples, t1)
        if a is None or b is None:
            return None
        total += b - a
    return total


def main():
    run_dir = Path(sys.argv[1])
    runs = list(csv.DictReader((run_dir / "runs.csv").open()))
    print("framework scenario half  | tester req/s | app rx pkt/req B/pkt | app tx pkt/req B/pkt | "
          "app CPU cadvisor system | tester CPU max")
    for run in runs:
        fw, scen = run["framework"], run["scenario"]
        start, stage_s, stages = int(run["start_at"]), int(run["stage_s"]), int(run["stages"])
        end = start + stages * stage_s
        span = end - start + 180
        # Deployment pods only (<fw>-<template hash>-<suffix>); "<fw>-get-xxxxx" tester pods must not match.
        pod = f'{fw}-[a-z0-9]{{6,10}}-[a-z0-9]{{5}}'
        tester = raw(f'tester_request_duration_seconds_count{{namespace="bench",framework="{fw}"}}', end + 60, span)
        tester = {k: v for k, v in tester.items() if json.loads(k).get("pod", "").startswith(f"{fw}-{scen}-")}

        def net(m):
            return raw(f'{m}{{namespace="bench",pod=~"{pod}",interface="eth0"}}', end + 60, span)
        rxb, rxp = net("container_network_receive_bytes_total"), net("container_network_receive_packets_total")
        txb, txp = net("container_network_transmit_bytes_total"), net("container_network_transmit_packets_total")
        cpu_c = raw(f'container_cpu_usage_seconds_total{{namespace="bench",container="app",pod=~"{pod}"}}', end + 60, span)
        cpu_s = raw(f'kubernetes_io:container_cpu_core_usage_time{{namespace_name="bench",container_name="app",pod_name=~"{pod}"}}', end + 120, span + 120)
        t_cpu = raw(f'kubernetes_io:container_cpu_core_usage_time{{namespace_name="bench",container_name="tester",pod_name=~"{fw}-{scen}-.*"}}', end + 120, span + 120)
        for name, (t0, t1) in {"0-9": (start + stage_s, start + 10 * stage_s), "10-19": (start + 10 * stage_s, end)}.items():
            dt = t1 - t0
            req = total_delta(tester, t0, t1, born_at_zero=True)
            d = [total_delta(x, t0, t1) for x in (rxp, rxb, txp, txb, cpu_c, cpu_s)]
            t_max = max((v / dt for x in t_cpu.values() if (v := total_delta({0: x}, t0, t1)) is not None), default=None)

            def f(v, spec):
                return "-" if v is None else format(v, spec)

            def ratio(a, b):
                return a / b if a is not None and b else None
            print(f"{fw:8} {scen:4} {name:5} | {f(ratio(req, dt), '12,.0f')} | "
                  f"{f(ratio(d[0], req), '10.3f')} {f(ratio(d[1], d[0]), '5.1f')} | "
                  f"{f(ratio(d[2], req), '10.3f')} {f(ratio(d[3], d[2]), '5.1f')} | "
                  f"{f(ratio(d[4], dt), '12.2f')} {f(ratio(d[5], dt), '6.2f')} | {f(t_max, '14.2f')}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Writes bench-dashboard.json, the Grafana dashboard for the GKE benchmark.

Only data from tester v2 runs is shown (series with a `run` label). Each panel
title states the caveat a reader needs; the description (i) explains the query.
The checks behind these choices are in results/gke-20260925-003821/validation.md.
"""
import json
from pathlib import Path

DEADLINE_LE = "1"  # tester DEADLINE_MS=1000 (gke/run.sh); must be a histogram bucket boundary
T = 'namespace="bench", run!=""'
APP = 'namespace="bench", container="app"'
FW_FROM_POD = '"framework", "$1", "pod", "(.+)-[a-z0-9]{6,10}-[a-z0-9]{5}"'
COLORS = {"stdlib": "#3987e5", "chi": "#d95926", "gin": "#199e70",
          "echo": "#c98500", "fiber": "#d55181", "fasthttp": "#008300"}


def app(expr):
    """Adds a framework label to a per-pod app series and sums per framework."""
    return f"sum by (framework) (label_replace({expr}, {FW_FROM_POD}))"


PANELS = [
    dict(title="p99 latency (dashed: 50 ms SLO)", unit="s", threshold=0.05,
         targets=[(f"histogram_quantile(0.99, sum by (framework, le) (rate(tester_request_duration_seconds_bucket{{{T}}}[30s])))",
                   "{{framework}}")],
         description="Measured by the load tester from each request's scheduled send time, so queueing in "
                     "an overloaded system is included (no coordinated omission). Requests that were never "
                     "sent because their 1 s deadline passed are not in this panel; see Availability."),
    dict(title="Requests per second (solid: 2xx, dashed: target)", unit="reqps",
         targets=[(f'sum by (framework) (rate(tester_request_duration_seconds_count{{{T}, status=~"2.."}}[30s]))', "{{framework}}"),
                  (f"sum by (framework) (tester_target_rps{{{T}}})", "{{framework}} target")],
         description="Solid: successful responses per second (30 s rate). Dashed: the scheduled rate. When "
                     "solid falls below dashed, the app is saturated. Cross-checked against packets on the "
                     "app pod's network interface (validation.md)."),
    dict(title="CPU usage (% of 2-CPU limit, 3m avg, ±5%)", unit="percent", threshold=100,
         targets=[(f"100 * {app(f'rate(container_cpu_usage_seconds_total{{{APP}}}[3m])')} / "
                   f"{app(f'container_spec_cpu_quota{{{APP}}} / container_spec_cpu_period{{{APP}}}')}", "{{framework}}")],
         description="cAdvisor sample timestamps lag the real measurement by 10-15 s, so short windows "
                     "over- and under-read. A 3-minute window keeps the error within about ±5%; readings "
                     "slightly above 100% are this sampling error, since the CFS quota cannot be exceeded. "
                     "Use CPU throttling to see exactly when the limit is hit."),
    dict(title="Memory usage (% of 256 MiB limit)", unit="percent",
         targets=[(f"100 * {app(f'container_memory_working_set_bytes{{{APP}}}')} / "
                   f"{app(f'container_spec_memory_limit_bytes{{{APP}}}')}", "{{framework}}")],
         description="Working set memory divided by the container memory limit. Matches the GKE system "
                     "metric kubernetes_io:container_memory_used_bytes."),
    dict(title="Availability (2xx within 1 s deadline, % of scheduled)", unit="percent", decimals=2,
         targets=[(f'100 * sum by (framework) (rate(tester_request_duration_seconds_bucket{{{T}, status=~"2..", le="{DEADLINE_LE}"}}[30s])) / '
                   f"(sum by (framework) (rate(tester_request_duration_seconds_count{{{T}}}[30s])) + "
                   f"sum by (framework) (rate(tester_dropped_requests_total{{{T}}}[30s])))", "{{framework}}")],
         description="Share of scheduled requests that got a 2xx response within 1 s of their scheduled "
                     "time. The denominator counts every scheduled request: sent (any outcome) plus "
                     "dropped (not sent because the deadline passed while every connection was busy)."),
    dict(title="CPU throttling (% of CFS periods)", unit="percent",
         targets=[(f"100 * {app(f'rate(container_cpu_cfs_throttled_periods_total{{{APP}}}[1m])')} / "
                   f"{app(f'rate(container_cpu_cfs_periods_total{{{APP}}}[1m])')}", "{{framework}}")],
         description="Share of 100 ms CFS periods in which the app used up its 2-CPU quota and was paused. "
                     "A ratio of two counters read together, so the cAdvisor timestamp lag cancels out; "
                     "it rises exactly where the app saturates."),
]


def overrides():
    out = []
    for fw, color in COLORS.items():
        out.append({"matcher": {"id": "byName", "options": fw},
                    "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": color}}]})
        out.append({"matcher": {"id": "byName", "options": f"{fw} target"},
                    "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": color}},
                                   {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}},
                                   {"id": "custom.lineWidth", "value": 1},
                                   {"id": "custom.fillOpacity", "value": 0}]})
    return out


def panel(i, p):
    defaults = {"unit": p["unit"], "custom": {"lineWidth": 2, "fillOpacity": 10, "showPoints": "never", "spanNulls": False}}
    if "decimals" in p:
        defaults["decimals"] = p["decimals"]
    if "threshold" in p:
        defaults["thresholds"] = {"mode": "absolute", "steps": [{"color": "transparent", "value": None},
                                                                {"color": "#8f8e88", "value": p["threshold"]}]}
        defaults["custom"]["thresholdsStyle"] = {"mode": "dashed"}
    ds = {"type": "prometheus", "uid": "gmp"}
    return {
        "id": i + 1, "type": "timeseries", "title": p["title"], "description": p["description"],
        "gridPos": {"h": 9, "w": 12, "x": 12 * (i % 2), "y": 9 * (i // 2)}, "datasource": ds,
        "targets": [{"refId": chr(65 + n), "expr": e, "legendFormat": leg, "datasource": ds}
                    for n, (e, leg) in enumerate(p["targets"])],
        "fieldConfig": {"defaults": defaults, "overrides": overrides()},
        "options": {"legend": {"displayMode": "table", "placement": "right", "calcs": ["lastNotNull", "max"]},
                    "tooltip": {"mode": "multi", "sort": "desc"}},
    }


dashboard = {"uid": "go-frameworks", "title": "Go frameworks benchmark (GKE)", "timezone": "utc",
             "time": {"from": "now-6h", "to": "now"}, "refresh": "30s", "schemaVersion": 39,
             "tags": ["benchmark"], "panels": [panel(i, p) for i, p in enumerate(PANELS)]}
Path(__file__).with_name("bench-dashboard.json").write_text(json.dumps(dashboard, indent=2) + "\n")

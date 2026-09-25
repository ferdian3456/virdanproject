#!/usr/bin/env python3
"""Compares CPU speed within and between machines from vmvar results.

Usage: experiments/vmvar/analyze.py results/vmvar-<run id> [more result dirs or out.jsonl files]

Every argument is a result directory (one sub-directory per machine with
out.jsonl and meta.json) or a single out.jsonl file. For each test it prints
each machine's median ns/op, its within-machine spread over the repetitions,
and the spread between machines (max/min - 1 of the medians).
"""
import json
import statistics
import sys
from pathlib import Path


def load(paths):
    machines = {}
    for p in map(Path, paths):
        files = [p] if p.is_file() else sorted(p.glob("*/out.jsonl"))
        for f in files:
            meta = f.with_name("meta.json")
            label = json.loads(meta.read_text())["cpu_platform"] if meta.exists() else ""
            rows = [json.loads(line) for line in f.read_text().splitlines() if line.strip()]
            machines[rows[0]["host"]] = (label, rows)
    return machines


def main():
    machines = load(sys.argv[1:])
    tests = sorted({r["test"] for _, rows in machines.values() for r in rows})
    for t in tests:
        print(f"== {t} (ns/op, lower is faster)")
        medians = {}
        for host, (label, rows) in sorted(machines.items()):
            v = [r["ns_per_op"] for r in rows if r["test"] == t]
            medians[host] = statistics.median(v)
            print(f"  {host:32} {label:20} median {medians[host]:10.3f}  within spread {(max(v) / min(v) - 1) * 100:5.2f}% (n={len(v)})")
        m = list(medians.values())
        print(f"  between machines: spread {(max(m) / min(m) - 1) * 100:.2f}%, "
              f"coefficient of variation {statistics.pstdev(m) / statistics.mean(m) * 100:.2f}% (n={len(m)})")


if __name__ == "__main__":
    main()

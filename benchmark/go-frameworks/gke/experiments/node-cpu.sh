#!/usr/bin/env bash
# Runs the vmvar CPU microbenchmark on both app nodes at the same time and saves
# its output to results/node-cpu-<run id>/<node>.jsonl. Run only when no benchmark
# is using the app nodes.
# Usage: gke/experiments/node-cpu.sh [run_id]
set -euo pipefail
cd "$(dirname "$0")/.."
RUN_ID=${1:-$(date -u +%Y%m%d-%H%M%S)}
OUT=../results/node-cpu-$RUN_ID
mkdir -p "$OUT"
out=$(./kctl.sh '
nodes=$(kubectl get nodes -l node=general -o jsonpath="{.items[*].metadata.name}")
i=0
for n in $nodes; do
  i=$((i+1))
  sed -e "s|__NODE__|$n|g; s|__NAME__|vmvar-$i|" k8s/experiments/vmvar.yaml.tmpl | kubectl apply -f - >/dev/null
done
kubectl -n bench wait --for=condition=complete job -l role=vmvar --timeout=600s >/dev/null 2>&1 ||
  kubectl -n bench wait --for=condition=complete job/vmvar-1 job/vmvar-2 --timeout=600s >/dev/null
for j in vmvar-1 vmvar-2; do kubectl -n bench logs job/$j | grep "^{"; done
kubectl -n bench delete job vmvar-1 vmvar-2 >/dev/null')
echo "$out" | grep '^{' | python3 -c '
import json, sys, collections
rows = [json.loads(l) for l in sys.stdin]
by = collections.defaultdict(list)
for r in rows: by[r["host"]].append(r)
for host, rs in by.items():
    open(sys.argv[1] + "/" + host + ".jsonl", "w").write("".join(json.dumps(r) + "\n" for r in rs))
print(len(rows), "rows from", len(by), "nodes")' "$OUT"

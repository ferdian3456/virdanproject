#!/usr/bin/env bash
# Runs the GKE benchmark protocol: the six frameworks in random pairs (one app per app node,
# both loaded at the same time), GET then POST. Writes the schedule to results/gke-<run id>/runs.csv.
# Usage: gke/run.sh [run_id]
set -euo pipefail
cd "$(dirname "$0")"
RUN_ID=${1:-$(date -u +%Y%m%d-%H%M%S)}
OUT=../results/gke-$RUN_ID

# Per framework: PODS tester pods, aggregate rate PODS*START_RPS .. PODS*(START_RPS+(STAGES-1)*STEP_RPS),
# i.e. 5k to 100k RPS in 5k steps. Each pod stays at or below 20k RPS.
PODS=5 START_RPS=1000 STEP_RPS=1000 STAGES=20 STAGE_S=30
LEAD_S=150   # Cloud Build round trip and pod start before stage 0
LINGER_S=60  # testers keep serving /metrics this long after the last stage

mkdir -p "$OUT"
echo "run_id,pair,framework,scenario,start_at,end_at,pods,start_rps_pod,step_rps_pod,stages,stage_s" > "$OUT/runs.csv"
./kctl.sh 'kubectl apply -f k8s/base.yaml'

frameworks=($(printf '%s\n' stdlib chi gin echo fiber fasthttp | shuf))
echo "order: ${frameworks[*]}"
for ((p = 0; p < 3; p++)); do
  a=${frameworks[2*p]} b=${frameworks[2*p+1]}
  echo "$(date -u +%T) pair $((p+1)): $a $b"
  ./kctl.sh "kubectl apply -f k8s/apps/$a.yaml -f k8s/apps/$b.yaml
kubectl -n bench rollout status deploy/$a --timeout=180s
kubectl -n bench rollout status deploy/$b --timeout=180s
kubectl -n bench get pods -o wide"

  for scenario in get post; do
    start=$(( $(date +%s) + LEAD_S )); end=$(( start + STAGES * STAGE_S ))
    rm -rf .jobs && mkdir .jobs
    for fw in $a $b; do
      sed -e "s|__NAME__|$fw-$scenario|; s|__FRAMEWORK__|$fw|; s|__PODS__|$PODS|g" \
          -e "s|__TEST_URL__|http://$fw.bench.svc.cluster.local:8080/api/devices|; s|__REQUEST__|$scenario|" \
          -e "s|__START_AT__|$start|; s|__START_RPS__|$START_RPS|; s|__STEP_RPS__|$STEP_RPS|" \
          -e "s|__STAGES__|$STAGES|; s|__STAGE_INTERVAL_S__|$STAGE_S|" k8s/tester.yaml.tmpl > .jobs/$fw.yaml
      echo "$RUN_ID,$((p+1)),$fw,$scenario,$start,$end,$PODS,$START_RPS,$STEP_RPS,$STAGES,$STAGE_S" >> "$OUT/runs.csv"
    done
    echo "$(date -u +%T) $scenario: start_at=$start end_at=$end"
    ./kctl.sh 'kubectl apply -f .jobs/ && sleep 30 && kubectl -n bench get pods -l role=tester -o wide'
    until [ "$(date +%s)" -ge $(( end + LINGER_S + 15 )) ]; do sleep 10; done
    ./kctl.sh "kubectl -n bench get jobs; kubectl -n bench delete job $a-$scenario $b-$scenario"
  done

  ./kctl.sh "kubectl delete -f k8s/apps/$a.yaml -f k8s/apps/$b.yaml --wait=true"
done
rm -rf .jobs
echo "$(date -u +%T) done: $OUT/runs.csv"

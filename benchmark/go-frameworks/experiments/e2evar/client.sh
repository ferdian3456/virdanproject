#!/bin/bash
# Startup script of the client VM: waits for job files in the bucket, runs the
# load tester against the job's app VM, snapshots /metrics after every stage and
# uploads the snapshots. A job file contains: <app name> <app internal IP>.
set -uo pipefail
B=$(curl -sf -H Metadata-Flavor:Google http://metadata.google.internal/computeMetadata/v1/instance/attributes/bucket)
gcloud storage cp "$B/bin/loadtester" /usr/local/bin/loadtester && chmod +x /usr/local/bin/loadtester
STAGES=9 STAGE_S=15
while true; do
  job=$(gcloud storage ls "$B/jobs/" 2>/dev/null | head -1)
  [ -n "$job" ] || { sleep 5; continue; }
  read -r name ip < <(gcloud storage cat "$job")
  until curl -sf "http://$ip:8080/healthz" >/dev/null; do sleep 2; done
  out=/tmp/$name; mkdir -p "$out"
  start=$(( $(date +%s) + 5 ))
  TEST_URL=http://$ip:8080/api/devices START_AT=$start START_RPS=5000 STEP_RPS=5000 STAGES=$STAGES \
    STAGE_INTERVAL_S=$STAGE_S CONNECTIONS=128 DEADLINE_MS=1000 METRICS_PORT=8085 \
    nohup /usr/local/bin/loadtester >"$out/tester.log" 2>&1 &
  pid=$!
  for ((k = 0; k <= STAGES; k++)); do
    sleep $(( start + k * STAGE_S - $(date +%s) )) 2>/dev/null || true
    curl -sf localhost:8085/metrics > "$out/snap_$k.txt"
  done
  sleep 3; curl -sf localhost:8085/metrics > "$out/snap_end.txt"
  kill $pid
  gcloud storage cp "$out"/* "$B/results/$name/" && gcloud storage rm "$job"
done

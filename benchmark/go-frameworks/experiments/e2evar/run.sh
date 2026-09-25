#!/usr/bin/env bash
# Measures VM-to-VM variability of end-to-end HTTP capacity: one fixed client VM
# loads COUNT fresh app VMs (stdlib app pinned to one vCPU), one at a time, with a
# 5k-45k RPS ramp.
# RUNS_PER_VM > 1 measures each app VM several times in a row, which separates
# VM-to-VM differences from run-to-run (time) variation.
# Usage: experiments/e2evar/run.sh [run_id]   (env: COUNT, RUNS_PER_VM, APP_MACHINE, CLIENT_MACHINE, ZONE)
set -euo pipefail
cd "$(dirname "$0")"
RUN_ID=${1:-$(date -u +%Y%m%d-%H%M%S)}
COUNT=${COUNT:-4} RUNS_PER_VM=${RUNS_PER_VM:-1} APP_MACHINE=${APP_MACHINE:-n2-standard-2} CLIENT_MACHINE=${CLIENT_MACHINE:-n2-standard-2}
ZONE=${ZONE:-us-central1-a}
G="env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud --project go-bench-15205 --quiet"
B=gs://go-bench-15205-results/e2evar/$RUN_ID
OUT=../../results/e2evar-$RUN_ID
mkdir -p "$OUT"
(cd ../../app && CGO_ENABLED=0 go build -o /tmp/e2e-stdlib ./cmd/stdlib)
(cd ../../loadtester && CGO_ENABLED=0 go build -o /tmp/e2e-loadtester .)
$G storage cp /tmp/e2e-stdlib "$B/bin/stdlib" >/dev/null && $G storage cp /tmp/e2e-loadtester "$B/bin/loadtester" >/dev/null
tag=$(echo "$RUN_ID" | tr -dc '0-9' | tail -c 7)
client=e2e-$tag-client app=""
cleanup() {
  for vm in $client $app; do [ -n "$vm" ] && $G compute instances delete "$vm" --zone "$ZONE" >/dev/null 2>&1; done
  return 0
}
trap cleanup EXIT
create() {  # create <name> <machine> <startup script>
  $G compute instances create "$1" --zone "$ZONE" --machine-type "$2" --image-family debian-12 \
    --image-project debian-cloud --scopes storage-rw --metadata "bucket=$B" \
    --metadata-from-file startup-script="$3" >/dev/null
}
create "$client" "$CLIENT_MACHINE" client.sh
echo "$(date -u +%T) client $client ($CLIENT_MACHINE, $ZONE)"
for ((i = 1; i <= COUNT; i++)); do
  app=e2e-$tag-app$i
  create "$app" "$APP_MACHINE" app.sh
  ip=$($G compute instances describe "$app" --zone "$ZONE" --format='value(networkInterfaces[0].networkIP)')
  platform=$($G compute instances describe "$app" --zone "$ZONE" --format='value(cpuPlatform)')
  for ((r = 1; r <= RUNS_PER_VM; r++)); do
    job=$app; [ "$RUNS_PER_VM" -gt 1 ] && job=$app-r$r
    echo "$job $ip" | $G storage cp - "$B/jobs/$job" >/dev/null
    deadline=$(( $(date +%s) + 900 ))
    until $G storage ls "$B/results/$job/snap_end.txt" >/dev/null 2>&1; do
      [ "$(date +%s)" -lt "$deadline" ] || { echo "$(date -u +%T) timeout $job"; exit 1; }
      sleep 15
    done
    mkdir -p "$OUT/$job" && $G storage cp "$B/results/$job/*" "$OUT/$job/" >/dev/null
    echo "{\"host\":\"$app\",\"run\":$r,\"cpu_platform\":\"$platform\",\"zone\":\"$ZONE\"}" > "$OUT/$job/meta.json"
  done
  $G compute instances delete "$app" --zone "$ZONE" >/dev/null
  echo "$(date -u +%T) done $app ($platform)"
  app=""
done

#!/usr/bin/env bash
# Measures VM-to-VM variability: creates COUNT fresh VMs of one type, one at a time,
# runs vmvar on each (startup.sh) and collects the results under results/vmvar-<run id>/.
# Usage: experiments/vmvar/run.sh [run_id]   (env: COUNT, MACHINE, ZONES)
# ZONES is tried in order for every VM (machine types are often out of stock in one zone).
# EXTRA_FLAGS is passed to "instances create", e.g. EXTRA_FLAGS="--threads-per-core=1".
set -euo pipefail
cd "$(dirname "$0")"
RUN_ID=${1:-$(date -u +%Y%m%d-%H%M%S)}
COUNT=${COUNT:-5} MACHINE=${MACHINE:-n2-standard-4} EXTRA_FLAGS=${EXTRA_FLAGS:-}
ZONES=${ZONES:-"us-central1-c us-central1-f us-central1-a us-east1-b us-east1-c us-east4-a us-west1-b"}
ZONE=
G="env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud --project go-bench-15205 --quiet"
B=gs://go-bench-15205-results/vmvar/$RUN_ID
OUT=../../results/vmvar-$RUN_ID
mkdir -p "$OUT"
CGO_ENABLED=0 go build -o /tmp/vmvar-bin . && $G storage cp /tmp/vmvar-bin "$B/vmvar" >/dev/null
name=""
# Never leave a VM running: delete the current one on any exit.
trap '[ -n "$name" ] && $G compute instances delete "$name" --zone "$ZONE" >/dev/null 2>&1 || true' EXIT
for ((i = 1; i <= COUNT; i++)); do
  name=vmvar-$(echo "$RUN_ID" | tr -dc '0-9' | tail -c 7)-$i
  ZONE=""
  for z in $ZONES; do
    if $G compute instances create "$name" --zone "$z" --machine-type "$MACHINE" \
      --image-family debian-12 --image-project debian-cloud --scopes storage-rw $EXTRA_FLAGS \
      --metadata "results=$B" --metadata-from-file startup-script=startup.sh >/dev/null 2>&1; then
      ZONE=$z; break
    fi
  done
  [ -n "$ZONE" ] || { echo "$(date -u +%T) no zone had capacity for $MACHINE"; name=""; exit 1; }
  echo "$(date -u +%T) created $name ($MACHINE $EXTRA_FLAGS, $ZONE)"
  deadline=$(( $(date +%s) + 600 ))
  until $G storage ls "$B/$name/meta.json" >/dev/null 2>&1; do
    [ "$(date +%s)" -lt "$deadline" ] || { echo "$(date -u +%T) timeout $name"; exit 1; }
    sleep 15
  done
  mkdir -p "$OUT/$name"
  $G storage cp "$B/$name/*" "$OUT/$name/" >/dev/null
  $G compute instances delete "$name" --zone "$ZONE" >/dev/null
  echo "$(date -u +%T) done $name: $(cat "$OUT/$name/meta.json")"
  name=""
done

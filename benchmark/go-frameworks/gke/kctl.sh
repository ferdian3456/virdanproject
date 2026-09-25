#!/usr/bin/env bash
# Runs shell commands (kubectl etc.) against the bench cluster from inside Cloud Build,
# because the session's egress proxy cannot reach the GKE API server.
# The gke/ directory is uploaded and used as the working directory.
# Usage: gke/kctl.sh 'kubectl apply -f k8s/base.yaml && kubectl get pods -n bench'
set -euo pipefail
dir=$(cd "$(dirname "$0")" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
cp -r "$dir"/. "$stage"/
printf '%s\n' "set -euo pipefail" "echo ===KCTL===" \
  "gcloud container clusters get-credentials bench --zone us-central1-b --quiet 2>/dev/null" \
  "$1" > "$stage/run.sh"
cat > "$stage/cloudbuild.yaml" <<'YAML'
steps:
  - name: gcr.io/cloud-builders/kubectl
    entrypoint: bash
    args: ['run.sh']
YAML
env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud builds submit "$stage" --config "$stage/cloudbuild.yaml" \
  --project go-bench-15205 --region us-central1 > "$stage/build.log" 2>&1 && rc=0 || rc=$?
# Print only the step's own output: from the marker to the end of the BUILD phase.
awk '/^===KCTL===$/{on=1;next} /^(PUSH|DONE)$/{on=0} on' "$stage/build.log"
grep '^ERROR' "$stage/build.log" || true
exit $rc

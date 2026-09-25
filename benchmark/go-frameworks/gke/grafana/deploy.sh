#!/usr/bin/env bash
# Deploys Grafana + datasource-syncer and checks every dashboard query through Grafana.
# Runs inside Cloud Build: gke/kctl.sh 'bash grafana/deploy.sh [verify_unix_time]'
# Secrets are generated here and never printed.
set -euo pipefail
VERIFY_AT=${1:-$(date +%s)}

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
kubectl -n monitoring get secret grafana-admin >/dev/null 2>&1 ||
  kubectl -n monitoring create secret generic grafana-admin --from-literal=password="$(openssl rand -base64 18)"
kubectl -n monitoring create configmap grafana-dashboards --from-file=grafana/bench-dashboard.json \
  --dry-run=client -o yaml | kubectl apply -f -
# Grafana reloads provisioned dashboards from the ConfigMap by itself (no restart needed).
# Its database is not persistent, so a restart would lose the syncer's service account.
kubectl apply -f grafana/grafana.yaml
kubectl -n monitoring rollout status deploy/grafana --timeout=180s

kubectl -n monitoring port-forward svc/grafana 3000:3000 >/dev/null 2>&1 &
trap 'kill %1' EXIT
until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 1; done
PW=$(kubectl -n monitoring get secret grafana-admin -o jsonpath='{.data.password}' | base64 -d)
api() { curl -sf -u "admin:$PW" -H 'Content-Type: application/json' "$@"; }

# (Re)create the syncer's token on every deploy, so a Grafana restart cannot leave a stale one.
sa_id=$(api "http://localhost:3000/api/serviceaccounts/search?query=datasource-syncer" |
  python3 -c 'import json,sys; a=json.load(sys.stdin)["serviceAccounts"]; print(a[0]["id"] if a else "")')
[ -n "$sa_id" ] || sa_id=$(api -d '{"name":"datasource-syncer","role":"Admin"}' http://localhost:3000/api/serviceaccounts |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
token=$(api -d "{\"name\":\"datasource-syncer-$(date +%s)\"}" "http://localhost:3000/api/serviceaccounts/$sa_id/tokens" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["key"])')
kubectl -n monitoring create secret generic datasource-syncer --from-literal=token="$token" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f grafana/datasource-syncer.yaml
kubectl -n monitoring delete job datasource-syncer-init --ignore-not-found
kubectl -n monitoring create job datasource-syncer-init --from=cronjob/datasource-syncer
kubectl -n monitoring wait --for=condition=complete job/datasource-syncer-init --timeout=180s
kubectl -n monitoring logs job/datasource-syncer-init | tail -3

# Every panel query through the Grafana data source proxy, at VERIFY_AT.
GRAFANA_PW="$PW" python3 grafana/verify.py "$VERIFY_AT"

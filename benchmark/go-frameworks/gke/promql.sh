#!/usr/bin/env bash
# Queries Google Cloud Managed Service for Prometheus with PromQL.
# Usage: gke/promql.sh '<query>' [start_unix end_unix step]
#   With only a query it runs an instant query; with start/end/step a range query.
set -euo pipefail
base=https://monitoring.googleapis.com/v1/projects/go-bench-15205/location/global/prometheus/api/v1
token=$(env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud auth print-access-token)
if [ $# -ge 4 ]; then
  curl -sS -H "Authorization: Bearer $token" "$base/query_range" \
    --data-urlencode "query=$1" -d "start=$2" -d "end=$3" -d "step=$4"
else
  curl -sS -H "Authorization: Bearer $token" "$base/query" --data-urlencode "query=$1"
fi

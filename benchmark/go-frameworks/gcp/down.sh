#!/usr/bin/env bash
# Deletes both benchmark VMs. The results bucket is kept.
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh

gcloud compute instances delete "$SUT_VM" "$LOADGEN_VM" --project="$PROJECT" --zone="$ZONE" --quiet

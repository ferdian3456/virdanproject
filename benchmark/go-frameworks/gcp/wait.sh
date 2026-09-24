#!/usr/bin/env bash
# Waits for a run to finish and downloads its results to ../results/<RUN_ID>.
# Usage: ./wait.sh <RUN_ID>
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh

RUN_ID=${1:?usage: ./wait.sh <RUN_ID>}
PREFIX="gs://$BUCKET/runs/$RUN_ID"

until gcloud storage ls "$PREFIX/DONE" >/dev/null 2>&1; do
	if gcloud storage ls "$PREFIX/FAILED" >/dev/null 2>&1; then
		echo "run $RUN_ID FAILED:"
		gcloud storage cat "$PREFIX/FAILED"
		gcloud storage cat "$PREFIX/loadgen.log" | tail -50
		exit 1
	fi
	sleep 60
done

sleep 35 # let the SUT push its last cpu.csv upload
dest=../results/$RUN_ID
mkdir -p "$dest"
gcloud storage cp -r "$PREFIX/results" "$PREFIX/sut" "$PREFIX/loadgen.log" "$dest/"
echo "results in $(cd "$dest" && pwd)"

#!/usr/bin/env bash
# Builds the framework binaries, uploads them with the bench scripts to GCS and
# creates the SUT and load generator VMs. The benchmark starts on boot.
#
# Usage: ./up.sh                        # full run
#        RATES="5000 10000" REPEATS=1 DURATION_S=20 ./up.sh   # smoke run
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh

RUN_ID=${RUN_ID:-$(date -u +%Y%m%d-%H%M%S)}
PREFIX="gs://$BUCKET/runs/$RUN_ID"

(
	cd ../app
	for fw in stdlib chi gin echo fiber fasthttp; do
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "../build/$fw" "./cmd/$fw"
	done
)

gcloud storage buckets describe "gs://$BUCKET" --project="$PROJECT" >/dev/null 2>&1 ||
	gcloud storage buckets create "gs://$BUCKET" --project="$PROJECT" --location="$REGION" --uniform-bucket-level-access
gcloud storage cp ../build/* "$PREFIX/bin/"
gcloud storage cp bench.sh post.lua "$PREFIX/scripts/"

common=(
	--project="$PROJECT" --zone="$ZONE"
	--image-family=debian-12 --image-project=debian-cloud --boot-disk-size=20GB
	--scopes=storage-rw
)

gcloud compute instances create "$SUT_VM" "${common[@]}" \
	--machine-type="$SUT_MACHINE" \
	--metadata="bucket=$BUCKET,run-id=$RUN_ID" \
	--metadata-from-file=startup-script=sut-startup.sh

sut_ip=$(gcloud compute instances describe "$SUT_VM" --project="$PROJECT" --zone="$ZONE" \
	--format='value(networkInterfaces[0].networkIP)')

meta="bucket=$BUCKET,run-id=$RUN_ID,sut-ip=$sut_ip"
for k in FRAMEWORKS SCENARIOS RATES REPEATS DURATION_S CONNECTIONS P99_SLO_MS; do
	if [ -n "${!k:-}" ]; then meta+=",$(echo "$k" | tr 'A-Z_' 'a-z-')=${!k}"; fi
done

gcloud compute instances create "$LOADGEN_VM" "${common[@]}" \
	--machine-type="$LOADGEN_MACHINE" \
	--metadata="$meta" \
	--metadata-from-file=startup-script=loadgen-startup.sh

echo "RUN_ID=$RUN_ID"
echo "Next: ./wait.sh $RUN_ID && ./down.sh"

#!/usr/bin/env bash
# GCE startup script for the load generator VM. Builds wrk2, runs bench.sh
# against the SUT and uploads the results to GCS, finishing with a DONE marker
# (or FAILED on error) that wait.sh polls for.
set -euo pipefail

md() { curl -sf -H 'Metadata-Flavor: Google' "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"; }
BUCKET=$(md bucket)
RUN_ID=$(md run-id)
PREFIX="gs://$BUCKET/runs/$RUN_ID"

# Startup scripts run on every boot; run the benchmark only once.
[ -e /opt/bench/started ] && exit 0
mkdir -p /opt/bench/results
touch /opt/bench/started
cd /opt/bench

exec > >(tee -a /opt/bench/loadgen.log) 2>&1
fail() {
	gcloud storage cp /opt/bench/loadgen.log "$PREFIX/loadgen.log" || true
	echo "$1" | gcloud storage cp - "$PREFIX/FAILED"
	exit 1
}
trap 'fail "failed at line $LINENO"' ERR

export SUT_HOST
SUT_HOST=$(md sut-ip)
# Optional bench.sh overrides passed as instance metadata, e.g. rates, repeats, duration-s.
for k in FRAMEWORKS SCENARIOS RATES REPEATS DURATION_S CONNECTIONS P99_SLO_MS; do
	v=$(md "$(echo "$k" | tr 'A-Z_' 'a-z-')" || true)
	if [ -n "$v" ]; then export "$k=$v"; fi
done

apt-get update -qq
apt-get install -y -qq build-essential libssl-dev zlib1g-dev git
git clone --depth 1 https://github.com/giltene/wrk2 /opt/wrk2
make -C /opt/wrk2 -j"$(nproc)"

gcloud storage cp "$PREFIX/scripts/bench.sh" "$PREFIX/scripts/post.lua" .

sysctl -w net.ipv4.ip_local_port_range="1024 65535" net.ipv4.tcp_tw_reuse=1
ulimit -n 1048576

# Push partial results periodically so progress is visible while the run is going.
(while true; do
	sleep 120
	gcloud storage rsync -r results "$PREFIX/results" --quiet || true
	gcloud storage cp loadgen.log "$PREFIX/loadgen.log" --quiet || true
done) &
sync_pid=$!

WRK=/opt/wrk2/wrk OUT_DIR=/opt/bench/results LUA_DIR=/opt/bench bash bench.sh

kill "$sync_pid"
gcloud storage rsync -r results "$PREFIX/results"
gcloud storage cp loadgen.log "$PREFIX/loadgen.log"
echo ok | gcloud storage cp - "$PREFIX/DONE"

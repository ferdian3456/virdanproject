#!/usr/bin/env bash
# GCE startup script for the system-under-test VM. Starts all six framework
# binaries on their own port (only one is under load at a time), samples their
# CPU time every second and keeps uploading the samples to GCS.
set -euo pipefail

md() { curl -sf -H 'Metadata-Flavor: Google' "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"; }
BUCKET=$(md bucket)
RUN_ID=$(md run-id)
PREFIX="gs://$BUCKET/runs/$RUN_ID"
FRAMEWORKS="stdlib chi gin echo fiber fasthttp" # ports 8081..8086, must match bench.sh

sysctl -w net.core.somaxconn=65535 net.ipv4.tcp_max_syn_backlog=65535
ulimit -n 1048576

mkdir -p /opt/bench
cd /opt/bench
gcloud storage cp "$PREFIX/bin/*" .
chmod +x $FRAMEWORKS

port=8081
for fw in $FRAMEWORKS; do
	nohup "./$fw" -addr ":$port" >"$fw.log" 2>&1 &
	echo $! >"$fw.pid"
	port=$((port + 1))
done

printf 'nproc=%s\nclk_tck=%s\nkernel=%s\n' "$(nproc)" "$(getconf CLK_TCK)" "$(uname -r)" >info.txt
gcloud storage cp info.txt "$PREFIX/sut/info.txt"

# Cumulative user+system CPU ticks per framework process (fields 14 and 15 of /proc/<pid>/stat).
echo "epoch,framework,cpu_ticks" >cpu.csv
while true; do
	ts=$(date +%s)
	for fw in $FRAMEWORKS; do
		read -r _ _ _ _ _ _ _ _ _ _ _ _ _ utime stime _ <"/proc/$(cat "$fw.pid")/stat" || continue
		echo "$ts,$fw,$((utime + stime))"
	done
	sleep 1
done >>cpu.csv &

# Keep the script in the foreground so the servers and the sampler are never reaped.
while true; do
	gcloud storage cp cpu.csv "$PREFIX/sut/cpu.csv" --quiet || true
	sleep 30
done

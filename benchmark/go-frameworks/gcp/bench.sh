#!/usr/bin/env bash
# Runs the wrk2 constant-rate step matrix against the SUT and writes the raw
# wrk2 output of every step to $OUT_DIR/<scenario>/<framework>/r<rep>/<rate>.txt.
# For each framework the rate is raised step by step until the step breaks the
# SLO (p99, achieved rate or error rate), then the next framework starts.
set -euo pipefail

: "${SUT_HOST:?SUT_HOST is required}"
WRK=${WRK:-wrk}
OUT_DIR=${OUT_DIR:-results}
FRAMEWORKS=${FRAMEWORKS:-"stdlib chi gin echo fiber fasthttp"}
SCENARIOS=${SCENARIOS:-"get post"}
RATES=${RATES:-"10000 20000 40000 60000 80000 100000 125000 150000 200000"}
REPEATS=${REPEATS:-3}
DURATION_S=${DURATION_S:-40} # wrk2 calibrates for the first ~10s; only the rest is recorded
WARMUP_S=${WARMUP_S:-15}
THREADS=${THREADS:-$(nproc)}
CONNECTIONS=${CONNECTIONS:-512}
P99_SLO_MS=${P99_SLO_MS:-50}
COOLDOWN_S=${COOLDOWN_S:-10}
LUA_DIR=${LUA_DIR:-$(dirname "$0")}

# Must match the ports in sut-startup.sh.
port_of() {
	case $1 in
	stdlib) echo 8081 ;;
	chi) echo 8082 ;;
	gin) echo 8083 ;;
	echo) echo 8084 ;;
	fiber) echo 8085 ;;
	fasthttp) echo 8086 ;;
	*) echo "unknown framework $1" >&2 && return 1 ;;
	esac
}

# to_ms converts a wrk2 latency value (950.00us, 1.25ms, 2.10s, 1.00m) to milliseconds.
to_ms() {
	awk -v v="$1" 'BEGIN {
		n = v + 0
		if (v ~ /us$/) n /= 1000
		else if (v ~ /ms$/) n = n
		else if (v ~ /m$/) n *= 60000
		else if (v ~ /s$/) n *= 1000
		printf "%.3f", n
	}'
}

wait_healthy() {
	local fw deadline=$((SECONDS + 600))
	for fw in $FRAMEWORKS; do
		until curl -sf "http://$SUT_HOST:$(port_of "$fw")/healthz" >/dev/null; do
			((SECONDS < deadline)) || { echo "$fw is not healthy after 600s" >&2; return 1; }
			sleep 5
		done
	done
	echo "all frameworks healthy"
}

wait_healthy
mkdir -p "$OUT_DIR"
[ -f "$OUT_DIR/runs.csv" ] || echo "rep,scenario,framework,rate,start_epoch,end_epoch" >"$OUT_DIR/runs.csv"

for rep in $(seq 1 "$REPEATS"); do
	for scenario in $SCENARIOS; do
		# Shuffle so no framework always runs first (cold) or last.
		for fw in $(printf '%s\n' $FRAMEWORKS | shuf); do
			url="http://$SUT_HOST:$(port_of "$fw")/api/devices"
			script=()
			[ "$scenario" = post ] && script=(-s "$LUA_DIR/post.lua")

			"$WRK" -t"$THREADS" -c"$CONNECTIONS" -d"${WARMUP_S}s" -R"${RATES%% *}" "${script[@]}" "$url" >/dev/null 2>&1 || true

			dir="$OUT_DIR/$scenario/$fw/r$rep"
			mkdir -p "$dir"
			for rate in $RATES; do
				out="$dir/$rate.txt"
				start=$(date +%s)
				"$WRK" -t"$THREADS" -c"$CONNECTIONS" -d"${DURATION_S}s" -R"$rate" --latency "${script[@]}" "$url" >"$out" 2>&1 || true
				end=$(date +%s)
				echo "$rep,$scenario,$fw,$rate,$start,$end" >>"$OUT_DIR/runs.csv"

				p99=$(awk '$1 == "99.000%" { print $2 }' "$out")
				rps=$(awk '/^Requests\/sec:/ { print $2 }' "$out")
				total=$(awk '/requests in/ { print $1 }' "$out")
				errs=$(awk '/Non-2xx/ { n += $NF }
					/Socket errors/ { for (i = 4; i <= NF; i += 2) { gsub(",", "", $i); n += $i } }
					END { print n + 0 }' "$out")
				p99_ms=$(to_ms "${p99:-0}")
				echo "rep=$rep scenario=$scenario fw=$fw rate=$rate rps=${rps:-0} p99_ms=$p99_ms errors=$errs/${total:-0}"

				broke=$(awk -v p="$p99_ms" -v slo="$P99_SLO_MS" -v rps="${rps:-0}" -v rate="$rate" \
					-v e="$errs" -v t="${total:-0}" -v has="${p99:+1}" \
					'BEGIN { print (has != 1 || p > slo || rps < 0.95 * rate || (t > 0 && e / t > 0.01)) ? 1 : 0 }')
				sleep "$COOLDOWN_S"
				[ "$broke" = 1 ] && break
			done
		done
	done
done
echo "benchmark finished"

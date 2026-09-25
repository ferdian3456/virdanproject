#!/bin/bash
# Startup script of one measurement VM: runs vmvar, uploads the results, powers off.
set -euo pipefail
md() { curl -sf -H Metadata-Flavor:Google "http://metadata.google.internal/computeMetadata/v1/instance/$1"; }
B=$(md attributes/results) NAME=$(md name) PLATFORM=$(md cpu-platform) ZONE=$(md zone)
gcloud storage cp "$B/vmvar" /tmp/vmvar && chmod +x /tmp/vmvar
lscpu > /tmp/lscpu.txt
/tmp/vmvar -reps 10 -dur 3s -host "$NAME" > /tmp/out.jsonl
printf '{"host":"%s","cpu_platform":"%s","zone":"%s"}\n' "$NAME" "$PLATFORM" "${ZONE##*/}" > /tmp/meta.json
gcloud storage cp /tmp/out.jsonl /tmp/lscpu.txt /tmp/meta.json "$B/$NAME/"
poweroff

#!/bin/bash
# Startup script of an app VM: runs the stdlib app on :8080, pinned to vCPU 0
# (GOMAXPROCS=1) so that it saturates well below what the client VM can send.
set -euo pipefail
B=$(curl -sf -H Metadata-Flavor:Google http://metadata.google.internal/computeMetadata/v1/instance/attributes/bucket)
gcloud storage cp "$B/bin/stdlib" /usr/local/bin/stdlib && chmod +x /usr/local/bin/stdlib
GOMAXPROCS=1 nohup taskset -c 0 /usr/local/bin/stdlib -addr :8080 >/var/log/stdlib.log 2>&1 &

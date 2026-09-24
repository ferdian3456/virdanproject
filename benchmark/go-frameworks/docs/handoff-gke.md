# Handoff: GKE Benchmark (Anton Putra Style, Open Model)

This document lets a new Claude Code session continue the Go framework benchmark on GKE. The previous session's container was replaced when the environment's network access was changed to **Full**, so every piece of state that is not in this repository or in GCP was lost.

## 1. Goal and decisions already made

The goal is to benchmark six Go HTTP stacks (`net/http` stdlib, chi, gin, echo, fiber and fasthttp) on **GKE**, in the style of Anton Putra's benchmarks (github.com/antonputra/tutorials, lessons 204, 229 and 275): apps run as Deployments with CPU limits, load generators run as Kubernetes Jobs on a separate node pool, and results come from Prometheus.

The following decisions were made by the user and should not be reopened:

| Topic | Decision |
|---|---|
| Load generator | Our own tool, `benchmark/go-frameworks/loadtester` (not k6, not Anton's image). |
| Load model | **Open model only** (constant arrival rate, like wrk2). This is intentionally *not* Anton's model: his tester is closed-loop (see the analysis document). |
| Ramp | Accelerated compared with Anton's multi-hour ramps. |
| Scenarios | `GET /api/devices` and `POST /api/devices`. |
| Environment | Network access set to Full. The session continues in a new container. |
| Cluster location | Move away from `us-central1-a` (see section 3). A different zone with N2 machines is preferred; a different machine type is the fallback. |
| Credentials | The user wanted to avoid logging in every session. Exporting the user's own gcloud login from a container is blocked by Claude Code's permission layer, so the user creates a service account key and stores it in the environment (section 4, option A). Never commit credentials to the repository. |

## 2. What already exists

### Repository (branch `claude/clone-antonputra-tutorials-x98933`)

| Path | Content |
|---|---|
| `benchmark/go-frameworks/app/` | Separate Go module with one `main.go` per framework under `cmd/<framework>`. All handlers share `internal/device` (same JSON encoding and validation, no middleware). The `-addr` flag sets the listen address. Ports used so far: 8081 stdlib, 8082 chi, 8083 gin, 8084 echo, 8085 fiber, 8086 fasthttp. |
| `benchmark/go-frameworks/gcp/` | The earlier **VM-based** run (wrk2 step load, startup scripts, `up.sh`, `wait.sh`, `down.sh`). |
| `benchmark/go-frameworks/results/20260924-102750/` | Results of the full VM run. Summary in section 5. |
| `benchmark/go-frameworks/docs/aputra-load-tester-analysis.md` | How Anton's `aputra/load-tester` behaves, from static inspection and black-box experiments. |
| `benchmark/go-frameworks/loadtester/` | Our open-model load tester (section 6). |

**Not written yet:** Dockerfiles for the apps and the load tester, Kubernetes manifests, and the GKE orchestration and analysis scripts.

### GCP (project `go-bench-15205`)

| Resource | State |
|---|---|
| Billing | Linked to the free trial billing account. A budget alert of IDR 800,000 at 50% and 90% exists. The billing account currency is **IDR**, so budgets must be created in IDR. |
| APIs enabled | compute, storage, billingbudgets, container, artifactregistry, cloudbuild. |
| Artifact Registry | Docker repository `bench` in `us-central1` (`us-central1-docker.pkg.dev/go-bench-15205/bench`). It is empty. |
| GCS bucket | `gs://go-bench-15205-results` (about 297 MB of earlier VM runs). It can be deleted at the end. |
| GKE cluster `bench` | **Zonal in `us-central1-a`. It must be deleted and recreated elsewhere.** It has one `e2-standard-4` node in `default-pool` (label `node=system`) and a pool `apps` (2× `n2-standard-4`) stuck in PROVISIONING because of `ZONE_RESOURCE_POOL_EXHAUSTED`. The `clients` pool was never created. Managed Service for Prometheus is enabled. |
| Quota | `CPUS_ALL_REGIONS` = 32. The regional quota for us-central1 includes N2 200, C2D 100, C3 24 and N2D 16. |

## 3. Known issues and environment quirks

1. **`CLOUDSDK_AUTH_ACCESS_TOKEN` is set in the environment** and overrides the logged-in account, which makes every `gcloud` call fail with `UNAUTHENTICATED`. Run gcloud as `env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud ...`. The variable's origin is unknown. Do not print or inspect its value.
2. **No Docker daemon** in the container. Build images with **Cloud Build** (`gcloud builds submit`), which pushes to Artifact Registry from inside GCP.
3. **Capacity stockouts in `us-central1-a`:** C2D was unavailable in every us-central1 zone this morning, and `n2-standard-4` was unavailable in `us-central1-a` this afternoon. Try other zones first and fall back to another machine type if needed. Keep the same machine type for every app node so results stay comparable.
4. With the earlier **Trusted** network level, the proxy blocked the GKE API server IP, `*.gke.goog`, `*.pkg.dev` and `dl.google.com`. Access is now **Full**. Verify with `kubectl get nodes` before doing anything else.
5. `kubectl` needs `gke-gcloud-auth-plugin`. The setup script below installs it. If it is missing, `gcloud components install gke-gcloud-auth-plugin` works now that `dl.google.com` is reachable.

### Recommended environment setup script

Paste this into **Edit cloud environment → Setup script** so every new session has the tools preinstalled:

```bash
#!/bin/bash
set -e
if [ ! -x /opt/google-cloud-sdk/bin/gcloud ]; then
  curl -sSfL https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz | tar -xz -C /opt
  /opt/google-cloud-sdk/bin/gcloud components install kubectl gke-gcloud-auth-plugin --quiet
fi
ln -sf /opt/google-cloud-sdk/bin/gcloud /opt/google-cloud-sdk/bin/gsutil \
       /opt/google-cloud-sdk/bin/kubectl /opt/google-cloud-sdk/bin/gke-gcloud-auth-plugin /usr/local/bin/
```

## 4. Logging in to gcloud

### Option A: service account key in the environment (no login per session)

The user runs these commands once in **Cloud Shell** (console.cloud.google.com, the `>_` icon):

```bash
PROJECT_ID=go-bench-15205
gcloud iam service-accounts create bench-runner --project=$PROJECT_ID --display-name="Benchmark runner"
SA=bench-runner@$PROJECT_ID.iam.gserviceaccount.com
# roles/editor is scoped to this benchmark-only project; container.admin adds GKE RBAC rights.
for ROLE in roles/editor roles/container.admin; do
  gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:$SA --role=$ROLE --condition=None
done
gcloud iam service-accounts keys create key.json --iam-account=$SA
base64 -w0 key.json; echo      # copy this single line
rm key.json
```

Then, in **Edit cloud environment → Environment variables**, add `GCP_SA_KEY_B64=<the base64 line>`. The dialog warns that environment variables are visible to anyone using the environment. That is acceptable only because the environment has one user and the key is limited to this project. Delete the key when the benchmark is finished (`gcloud iam service-accounts keys list/delete --iam-account=$SA`).

Append this to the setup script so every session is logged in automatically:

```bash
if [ -n "$GCP_SA_KEY_B64" ]; then
  echo "$GCP_SA_KEY_B64" | base64 -d > /root/gcp-sa-key.json
  chmod 600 /root/gcp-sa-key.json
  env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud auth activate-service-account --key-file=/root/gcp-sa-key.json
  env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud config set project go-bench-15205
fi
```

### Option B: user login with the link flow (once per session)

The flow must keep `gcloud` waiting for the code across turns, so feed its stdin from a FIFO:

```bash
S=<scratchpad dir>
mkfifo $S/authin
(sleep 100000 > $S/authin &)          # keep the FIFO open
env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud auth login --no-launch-browser --update-adc \
  < $S/authin > $S/auth.log 2>&1       # run with run_in_background
# Read the URL from $S/auth.log and send it to the user.
# After the user pastes the code:
echo '<code>' > $S/authin
env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud config set project go-bench-15205
```

The flow uses PKCE, so the code the user pastes is useless without the verifier held by the waiting process. Note that `gcloud auth login --cred-file` does not accept `authorized_user` files, so a user login cannot be restored from the ADC file.

## 5. Results so far (VM run, for reference)

The VM run used an `n2-highcpu-4` server VM and an `n2-highcpu-16` wrk2 VM, stepped the rate from 10k to 200k RPS, and did 3 repetitions. It passed the SLO when p99 ≤ 50 ms, the achieved rate was ≥ 95% of the target and errors were ≤ 1%. The max rate within SLO was identical in all three repetitions:

| Framework | GET | POST | GET p99 @ 60k | GET CPU @ 60k |
|---|---|---|---|---|
| fasthttp | 150k | 125k | 1.99 ms | 1.50 cores |
| fiber | 150k | 125k | 2.00 ms | 1.53 cores |
| stdlib | 80k | 60k | 3.86 ms | 2.41 cores |
| gin | 80k | 60k | 3.94 ms | 2.42 cores |
| chi | 80k | 60k | 4.01 ms | 2.50 cores |
| echo | 80k | 60k | 3.91 ms | 2.38 cores |

## 6. Load tester (`benchmark/go-frameworks/loadtester`)

The tester follows the wrk2 design. `CONNECTIONS` workers each own one keep-alive connection (fasthttp client) and send requests on a fixed schedule (`rate / CONNECTIONS` per worker). If a response is late, the next request is sent immediately and its latency still counts from the scheduled time. All pods of a run share `START_AT`, so their stages align in wall-clock time.

| Env var | Default | Meaning |
|---|---|---|
| `TEST_URL` | required | Target URL. |
| `REQUEST` | `get` | `get` or `post`. POST sends `{"mac":"EF-2B-C4-F5-D6-34","firmware":"2.1.5"}`. |
| `START_AT` | now | Unix seconds when stage 0 starts. Use the same value for every pod. |
| `START_RPS` / `STEP_RPS` | 100 / 100 | **Per-pod** rate at stage 0 and increase per stage. |
| `STAGES` / `STAGE_INTERVAL_S` | 10 / 30 | Number of stages and stage length. |
| `CONNECTIONS` | 128 | Connections (workers) per pod. |
| `TIMEOUT_MS` | 5000 | Request timeout. |
| `METRICS_PORT` | 8085 | Prometheus endpoint `/metrics`. |

The tester exposes these metrics:

- `tester_request_duration_seconds{method,status}`: a histogram with the same 261 buckets as Anton's tester, checked by `buckets_test.go`. The `status` label is the HTTP code, `timeout` or `error`, and failures keep their real latency.
- `tester_target_rps`: the scheduled per-pod rate.

Validation was done locally with one core, 128 connections, against the stdlib app pinned to 2 cores:

| Rate | Our p99 | wrk2 p99 |
|---|---|---|
| 5k | 2.0 ms | 3.1 ms |
| 20k | 8.5 ms | 7.1 ms |
| 40k | 10 ms | 11.6–12.3 ms |

The target rate was achieved in every case. **Keep each client pod (1 CPU) at or below about 20k RPS** for margin, and scale the pod count instead.

## 7. Remaining plan

Each step lists how to verify it.

1. **Tools and login.** Confirm that `gcloud`, `kubectl` and the auth plugin are installed. If `GCP_SA_KEY_B64` was set, the setup script has already logged in; otherwise use option B in section 4.
   *Verify:* `env -u CLOUDSDK_AUTH_ACCESS_TOKEN gcloud projects list` shows `go-bench-15205`.
2. **Recreate the cluster.** Delete `bench` in `us-central1-a`. Create a zonal Standard cluster in another zone (try `us-central1-b`, `-c`, `-f`) with `--enable-managed-prometheus`, a system pool (`e2-standard-4` ×1, label `node=system`), `apps` (`n2-standard-4` ×2, label `node=general`) and `clients` (`n2-highcpu-16` ×1, label `node=clients`). The total is 28 vCPU, within the 32 quota. If N2 is out of stock everywhere, pick one other type for `apps` and state that the results are not comparable with the VM run.
   *Verify:* `kubectl get nodes -L node` shows 4 Ready nodes with the right labels.
3. **Images.** Write a multi-stage Dockerfile for the apps (build argument selects `cmd/<framework>`, static binary, distroless or scratch base) and one for the load tester. Build all 7 images with Cloud Build into `us-central1-docker.pkg.dev/go-bench-15205/bench`.
   *Verify:* `gcloud artifacts docker images list` shows 7 images.
4. **Manifests,** mirroring Anton's lesson 229 (`lessons/229/deploy/*`):
   - One Deployment and one ClusterIP Service per framework on `node=general`.
   - Resources: requests `cpu: 1500m`, limits `cpu: 2000m`, `memory: 256Mi`. Set `GOMAXPROCS` from `limits.cpu` through `resourceFieldRef`.
   - One pod per node, so two frameworks run at the same time (one per app node).
   - Load tester Jobs on `node=clients`, one CPU per pod.
   - GMP `PodMonitoring` resources for the testers on port 8085.
   - Enable kubelet and cAdvisor scraping in the GMP `OperatorConfig` so container CPU and throttling metrics are collected. This has not been verified yet; check the current GMP documentation.
   *Verify:* the metrics appear through the GMP PromQL API.
5. **Test protocol.** Run frameworks in 3 pairs at the same time (one per app node), first GET and then POST.
   - The ramp is aggregate RPS across tester pods, for example 5k to about 100k in 30-second stages, with enough tester pods that each stays at or below 20k RPS. On a 2-CPU limit, expect roughly half of the VM run's capacity: about 40k for `net/http` stacks and 75k for fasthttp stacks. This is an estimate.
   - Randomize the pair order.
   *Verify:* `tester_target_rps` summed across pods matches the plan.
6. **Collect and analyze.** Query GMP with the PromQL HTTP API (`https://monitoring.googleapis.com/v1/projects/go-bench-15205/location/global/prometheus/api/v1/query_range`, using a bearer token from gcloud). Queries:
   - p50, p90 and p99 with `histogram_quantile` over `sum by (le)`.
   - Achieved RPS from `rate(..._count{status=~"2.."})`.
   - Errors.
   - Container CPU usage divided by the limit.
   - CPU throttling.
   Save the results as CSV under `benchmark/go-frameworks/results/gke-<run id>/`, and produce charts and a summary comparable with Anton's dashboards.
7. **Teardown.** Delete the cluster, then optionally the bucket and the Artifact Registry images.
   *Verify:* `gcloud container clusters list` and `gcloud compute instances list` are empty.

## 8. Open questions

- The exact GMP configuration for kubelet and cAdvisor metrics on the current GKE version.
- Whether `n2-standard-4` is available in another us-central1 zone at the time of the run.

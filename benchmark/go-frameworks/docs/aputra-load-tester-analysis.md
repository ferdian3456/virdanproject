# Analysis of `aputra/load-tester`

This document records how the load generator used in Anton Putra's benchmarks (`aputra/load-tester:v26`, the image referenced by lessons 204, 229, 275 and others in github.com/antonputra/tutorials) behaves. Its source code is not public. The goal is to reproduce its load model faithfully in our own implementation and to understand how it affects the numbers it reports.

No code from the binary is reproduced here. Everything below comes from two sources:

1. **Static inspection** of the public image: image metadata, symbol names (the binary is not stripped), embedded strings and call sites in the disassembly.
2. **Behavioral experiments**: running the extracted binary against a local observation server that logs every request, and scraping the tester's own `/metrics` endpoint.

Each finding is marked as **Confirmed** (observed directly in an experiment or stated unambiguously by the binary) or **Inferred** (consistent with the evidence but not directly observed).

## Build and dependencies (Confirmed)

- Written in Rust. The image copies `/app/target/release/load-tester` to `/server` and runs it as the default command. The v26 amd64 image was built on 2024-11-17.
- HTTP client: `reqwest` on top of `hyper`, `hyper-util` and `h2`, with TLS through `rustls` (OpenSSL is also linked).
- Async runtime: `tokio` (multi-threaded).
- Metrics: the `prometheus_client` crate, served by a minimal hand-written HTTP responder on `0.0.0.0:8085`.
- Configuration: `Tester.toml` parsed with `toml_edit` and `serde`. Randomness comes from `rand` (`thread_rng`).

## Inputs (Confirmed)

| Input | Meaning |
|---|---|
| `TEST_URL` environment variable | Target URL. The process exits with "The `TEST_URL` env variable is missing" if it is unset. |
| `Tester.toml` in the working directory | A `[test]` table with exactly seven fields: `request`, `protocol`, `min_clients`, `max_clients`, `stage_interval_s`, `request_delay_ms` and `request_timeout_ms`. |
| `ca.pem` in the working directory | Extra root certificate for HTTPS targets. Anton mounts it at `/ca.pem`, and the working directory is `/`. |

Values of `request` seen in the lesson manifests are `get`, `post`, `get-rest`, `post-rest`, `get-graphql` and `post-graphql`. Values of `protocol` are `http1` and `http2`. Only `get`, `post` and `http1` were exercised in these experiments.

## Requests (Confirmed)

- `GET` sends headers `Accept: */*` and `Accept-Encoding: gzip,deflate`.
- `POST` additionally sends `Content-Type: application/json` and the fixed body `{"mac":"EF-2B-C4-F5-D6-34","firmware":"2.1.5"}`.
- All clients share one HTTP client and one connection pool. Connections are kept alive and reused across clients. For example, one TCP connection served 93 requests coming from up to four clients.

## Load model

### Ramp (Confirmed)

- The test starts with `p = min_clients`.
- Every `stage_interval_s` seconds, `p` increases by one, and the tool prints `p is increased <p>` followed by `stage interval <s>`.
- **`max_clients` does not cap the ramp.** With `min_clients = 3` and `max_clients = 5`, `p` kept growing to 17 and beyond, and requests kept flowing. The only observed effect of `max_clients` is that **no load at all is generated when `min_clients == max_clients`**. The test therefore runs until the pod is stopped.
- `tester_active_clients` reports the current `p`.

### Batch execution with a barrier (Confirmed)

The symbol types show the core loop is built as:

```
stream::iter(urls).map(<spawn one request>).buffer_unordered(p).for_each(<record result>)
```

Behaviorally, requests are issued in **batches of `p`**, and a new batch starts only after **every** request of the previous batch has finished. Each request first sleeps a uniformly random time in `[0, request_delay_ms)`, then sends the request and measures its duration.

The resulting throughput matches the batch model almost exactly. The measurement used a local server with no delay and `request_delay_ms = 50`:

| `p` | Measured RPS | Batch model `p / E[max of p uniform delays]` | Independent-client model `p / (delay/2)` |
|---|---|---|---|
| 1 | 38.2 | 40 | 40 |
| 2 | 59.2 | 60 | 80 |
| 4 | 98.8 | 100 | 160 |
| 8 | 178.6 | 180 | 320 |
| 16 | 336.8 | 340 | 640 |
| 32 | 651.6 | 660 | 1280 |

For large `p`, one pod therefore produces roughly `(p + 1) / (request_delay_ms / 1000 + latency)` requests per second.

### Threads (Confirmed; mechanism Inferred)

- The per-request task calls `nanosleep` directly, so the random delay is a blocking sleep.
- The number of OS threads grows with `p`: 9 threads at `p = 2`, 39 at `p = 32` and 135 at `p = 128`. Five of them are tokio runtime threads and the rest are named after the process. Each concurrent slot therefore appears to get its own OS thread, which is why the blocking sleep does not cap throughput. The exact mechanism is inferred.

## Metrics (Confirmed)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `tester_active_clients` | gauge | none | Current `p`. |
| `tester_request_duration_seconds` | histogram | `method`, `status` | 261 fixed buckets from 10 µs to 5 s, identical to the buckets in Anton's open-source Go client (`lessons/258/client/metrics.go`). |

Measurement quirks that affect reported results:

1. **The `method` label is always `get`**, even for `POST` requests. They can be told apart only by status code, for example `201` for created devices.
2. **Every failed request is recorded with `status="408"` and a duration of `0`.** This applies to real timeouts and also to other client errors such as "error sending request". The log line is `TIMEOUT!!! record Got a reqwest::Error: ...`. As a result, failures never raise latency percentiles; they pull them down, and they show up only as reduced availability.
3. Latency is measured from just before sending until the response completes. It excludes the random delay.

## Implications for interpreting Anton's results

- **Strong coordinated omission.** Because each batch waits for its slowest request, one slow response stalls the next `p` requests. When the server slows down, the offered load drops at the same time, so reported tail latency near saturation is optimistic.
- **Load is set by `p` and the pod count, not by a target rate.** With the lesson 229 settings (`request_delay_ms = 40`, 29 pods), throughput grows by about 25 RPS per pod each time `p` increases.
- **Failures are invisible in latency panels.** They must be read from the availability panel (the share of non-4xx/5xx responses).

## Open questions (Unknown)

- The exact behavior of `protocol = "http2"` against plaintext targets (prior knowledge or upgrade) was not tested.
- The cause of occasional "error sending request" failures against a local server with zero delay was not identified.
- It is unknown whether versions other than v26 behave the same way.

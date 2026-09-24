# Benchmark summary: 20260924-101653

SLO: p99 <= 50 ms, achieved >= 95% of target rate, errors <= 1%.

## GET /api/devices

### Max rate within SLO (median across repetitions)

| Framework | Max rate (RPS) |
|---|---|
| fiber | 10,000 |
| stdlib | 10,000 |

### Latency and CPU per step (median across repetitions)

| Rate | Framework | Achieved RPS | p50 ms | p90 ms | p99 ms | p99.9 ms | CPU cores | Passed reps |
|---|---|---|---|---|---|---|---|---|
| 5,000 | fiber | 4,988 | 0.90 | 1.46 | 1.88 | 2.07 | 0.22 | 1/1 |
| 5,000 | stdlib | 4,966 | 0.92 | 1.46 | 1.96 | 2.55 | 0.38 | 1/1 |
| 10,000 | fiber | 9,969 | 0.83 | 1.35 | 1.83 | 2.07 | 0.38 | 1/1 |
| 10,000 | stdlib | 9,955 | 0.89 | 1.44 | 1.89 | 2.56 | 0.69 | 1/1 |

## POST /api/devices

### Max rate within SLO (median across repetitions)

| Framework | Max rate (RPS) |
|---|---|
| fiber | 10,000 |
| stdlib | 10,000 |

### Latency and CPU per step (median across repetitions)

| Rate | Framework | Achieved RPS | p50 ms | p90 ms | p99 ms | p99.9 ms | CPU cores | Passed reps |
|---|---|---|---|---|---|---|---|---|
| 5,000 | fiber | 4,988 | 0.88 | 1.40 | 1.86 | 2.06 | 0.22 | 1/1 |
| 5,000 | stdlib | 4,997 | 0.90 | 1.45 | 1.91 | 2.12 | 0.44 | 1/1 |
| 10,000 | fiber | 9,969 | 0.82 | 1.38 | 1.87 | 2.07 | 0.42 | 1/1 |
| 10,000 | stdlib | 9,954 | 0.89 | 1.45 | 1.90 | 2.34 | 0.78 | 1/1 |

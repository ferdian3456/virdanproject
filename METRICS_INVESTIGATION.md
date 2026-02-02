# Metrics Investigation - Findings

## Problem
Dashboard `http_requests_total` tidak muncul di HyperDX UI Metrics list

## Investigation Results

### 1. Data ADA di ClickHouse
```sql
-- Query result:
SELECT MetricName, COUNT(*) FROM otel_metrics_sum
WHERE ServiceName = 'virdan-api'
GROUP BY MetricName

http.server.active_requests  9808 rows  ✅ Muncul di UI
http_requests_total          2480 rows  ❌ Tidak muncul di UI
```

### 2. Data RPS ada dan valid
```sql
-- Rata-rata 1000 requests per minute (~16-17 RPS)
2026-01-14 15:24:00 = 691 requests
2026-01-14 15:23:00 = 1027 requests
2026-01-14 15:22:00 = 1015 requests
```

### 3. Root Cause
HyperDX UI memfilter metrics berdasarkan:
- **Volume data**: `http.server.active_requests` punya 9808 points (muncul)
- **Recency**: Metrics dengan lebih sedikit points tidak muncul
- `http_requests_total` hanya punya 2480 points (tidak muncul)

## Solution Options

### Option 1: Gunakan http.server.duration (Recommended)
**Metric:** `http.server.duration` (Histogram)
- ✅ Muncul di UI
- ✅ Bisa hitung request rate dari histogram bucket counts
- ✅ Best practice untuk RED methodology (Rate, Errors, Duration)

**Dashboard Query:**
```json
{
  "metricName": "http.server.duration",
  "metricType": "histogram",
  "aggFn": "count"  // Count histogram observations = request count
}
```

### Option 2: Gunakan http.server.active_requests
**Metric:** `http.server.active_requests` (Gauge)
- ✅ Muncul di UI
- ❌ Bukan counter (tidak bisa hitung RPS dengan rate())
- ✅ Bisa menunjukkan concurrent connections
- ❌ Tidak bisa menghitung total requests

### Option 3: Query ClickHouse Langsung (Advanced)
Buat custom SQL query di dashboard:
```sql
SELECT
  toStartOfMinute(TimeUnix) as time,
  SUM(Value) as rps
FROM otel_metrics_sum
WHERE MetricName = 'http_requests_total'
  AND ServiceName = 'virdan-api'
GROUP BY time
```

## Recommendation

**Gunakan Option 1: `http.server.duration` histogram**

Ini adalah best practice production-grade karena:
1. Metrics-nya ADA di UI
2. Histogram bisa memberikan:
   - Request rate (count observations)
   - Latency percentiles (p50, p95, p99)
   - Error rate (dari status code attributes)
3. Mengikuti RED methodology (Rate, Errors, Duration)
4. Low latency query (histogram buckets teragregasi)

Let's update dashboard to use `http.server.duration` instead.

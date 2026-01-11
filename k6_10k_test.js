import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    // Spike test ke 10k RPS
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 10000, // 10,000 RPS target
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 1000,  // Prepare 1000 VUs
      maxVUs: 2000,            // Max 2000 VUs
      exec: 'healthTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100'],   // 95% harus < 100ms
    http_req_failed: ['rate<0.05'],      // Max 5% error rate
  },
};

const BASE_URL = 'http://localhost:8081';

export function healthTest() {
  // Test health endpoint - super fast
  const response = http.get(`${BASE_URL}/api/health`, {
    tags: { name: 'HealthCheck' },
  });

  // Validate response
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  // No sleep - maximum pressure
}

import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    // Extreme spike test - 100k RPS
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 100000, // 100,000 RPS - EXTREME!
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 5000,  // Prepare 5000 VUs
      maxVUs: 15000,           // Max 15000 VUs
      exec: 'healthTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],   // 95% harus < 500ms (lebih realistis)
    http_req_failed: ['rate<0.1'],      // Max 10% error rate (acceptable untuk extreme load)
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
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  // No sleep - maximum pressure
}

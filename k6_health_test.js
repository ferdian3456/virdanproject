import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  // Load test sedang
  stages: [
    { duration: '30s', target: 50 },   // Ramp up ke 50 users dalam 30 detik
    { duration: '1m', target: 50 },     // Stay di 50 users selama 1 menit
    { duration: '20s', target: 0 },     // Ramp down ke 0 users dalam 20 detik
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],   // 95% request harus selesai < 500ms
    http_req_failed: ['rate<0.01'],     // Error rate harus < 1%
  },
};

const BASE_URL = 'http://localhost:8081';

export default function () {
  // Test health endpoint
  const response = http.get(`${BASE_URL}/api/health`);

  // Validate response
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  // Think time antar request (1-3 detik)
  sleep(Math.random() * 2 + 1);
}

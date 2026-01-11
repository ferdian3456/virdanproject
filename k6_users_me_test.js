import http from 'k6/http';
import { check, sleep } from 'k6';

// Access tokens dari 5 akun
const ACCESS_TOKENS = [
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2Nzc0NTg4MS1kYjViLTQxZjMtODEwNy0wNmM1YTdhNTAxYzUiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOjY3NzQ1ODgxLWRiNWItNDFmMy04MTA3LTA2YzVhN2E1MDFjNSIsImV4cCI6MTc2ODA2OTc5MSwibmJmIjoxNzY4MDY4ODkxLCJpYXQiOjE3NjgwNjg4OTF9.ExE57PMrZ7lYB_8UMoRI7l16XR_-RSvllWwjnEHPtoA',
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJlYWQ2NTY1ZS1mMDMzLTRkNTgtYTg0ZS1kOTgzYjA0N2ZjNGQiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOmVhZDY1NjVlLWYwMzMtNGQ1OC1hODRlLWQ5ODNiMDQ3ZmM0ZCIsImV4cCI6MTc2ODA2OTc5MSwibmJmIjoxNzY4MDY4ODkxLCJpYXQiOjE3NjgwNjg4OTF9.2IBQ7B_eRgaxxgTfolcC4mh96PQgIDo8f-p2jBmcxmA',
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJiNWNmNzFmZC03ZTkwLTQwZWUtOTA0YS02NDAxOTk0YTYzNmUiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOmI1Y2Y3MWZkLTdlOTAtNDBlZS05MDRhLTY0MDE5OTRhNjM2ZSIsImV4cCI6MTc2ODA2OTc5MiwibmJmIjoxNzY4MDY4ODkyLCJpYXQiOjE3NjgwNjg4OTJ9.D5wItD830gskHdLHllDbpF8P0rYCsl0sf4QLsl0iptw',
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiIzZTIzN2ZlZi0wZmMxLTQxZDctYTFkZC01ODA2NDVjYzYyYTAiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOjNlMjM3ZmVmLTBmYzEtNDFkNy1hMWRkLTU4MDY0NWNjNjJhMCIsImV4cCI6MTc2ODA2OTc5MiwibmJmIjoxNzY4MDY4ODkyLCJpYXQiOjE3NjgwNjg4OTJ9.Gr5ztNjwULF7p9vKROngmrm-ECprcYXsBNWzH5iXYsI',
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJlMzMwNjIzNi01OWE4LTRjYzQtYWQ4Zi03ZWQ1YTdiMWJhMDkiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOmUzMzA2MjM2LTU5YTgtNGNjNC1hZDhmLTdlZDVhN2IxYmEwOSIsImV4cCI6MTc2ODA2OTc5MiwibmJmIjoxNzY4MDY4ODkyLCJpYXQiOjE3NjgwNjg4OTJ9.RyaEOXvwMEe9cLr5q_pXjyaclSyuZ0y4dIiDx0Hd4IY',
];

export const options = {
  scenarios: {
    // Load test sedang untuk /api/users/me (ada database query)
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 1000,  // 1000 RPS target (mulai sedang dulu)
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 100,
      maxVUs: 500,
      exec: 'usersMeTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200'],   // 95% harus < 200ms
    http_req_failed: ['rate<0.01'],      // Max 1% error rate
  },
};

const BASE_URL = 'http://localhost:8081';

export function usersMeTest() {
  // Random pilih salah satu dari 5 token
  const randomToken = ACCESS_TOKENS[Math.floor(Math.random() * ACCESS_TOKENS.length)];

  // Test /api/users/me endpoint
  const response = http.get(`${BASE_URL}/api/users/me`, {
    headers: {
      'Authorization': `Bearer ${randomToken}`,
      'Content-Type': 'application/json',
    },
    tags: { name: 'GetUserMe' },
  });

  // Validate response
  check(response, {
    'status is 200': (r) => r.status === 200,
    'has user data': (r) => r.json('id') !== undefined,
    'response time < 200ms': (r) => r.timings.duration < 200,
  });

  // No sleep - maximum pressure
}

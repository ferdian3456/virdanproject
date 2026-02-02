import http from 'k6/http';
import { check, sleep } from 'k6';

// Access tokens dari 4 akun (updated 2026-01-17 - fresh tokens)
const ACCESS_TOKENS = [
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2Nzc0NTg4MS1kYjViLTQxZjMtODEwNy0wNmM1YTdhNTAxYzUiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOjY3NzQ1ODgxLWRiNWItNDFmMy04MTA3LTA2YzVhN2E1MDFjNSIsImV4cCI6MTc2ODU4NDY2MSwibmJmIjoxNzY4NTgzNzYxLCJpYXQiOjE3Njg1ODM3NjF9.WTfuYyrEJ4SNyDYhHjUGWm-i8L9rqMaUlpXd-dbCmA0', // ferdiandika3456
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJlYWQ2NTY1ZS1mMDMzLTRkNTgtYTg0ZS1kOTgzYjA0N2ZjNGQiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOmVhZDY1NjVlLWYwMzMtNGQ1OC1hODRlLWQ5ODNiMDQ3ZmM0ZCIsImV4cCI6MTc2ODU4NDY2MiwibmJmIjoxNzY4NTgzNzYyLCJpYXQiOjE3Njg1ODM3NjJ8.gY-vp3M4W7qhFVRqzwQrQY3ZmQO2YvP9h8uXLqDJ1oA', // ferdiandikaid
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJiNWNmNzFmZC03ZTkwLTQwZWUtOTA0YS02NDAxOTk0YTYzNmUiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOmI1Y2Y3MWZkLTdlOTAtNDBlZS05MDRhLTY0MDE5OTRhNjM2ZSIsImV4cCI6MTc2ODU4NDY2MiwibmJmIjoxNzY4NTgzNzYyLCJpYXQiOjE3Njg1ODM3NjJ8.gY-vp3M4W7qhFVRqzwQrQY3ZmQO2YvP9h8uXLqDJ1oA', // differentfire74
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiIzZTIzN2ZlZi0wZmMxLTQxZDctYTFkZC01ODA2NDVjYzYyYTAiLCJpc3MiOiJnaXRodWIuY29tL2ZlcmRpYW4zNDU2L3ZpcmRhbnByb2plY3QiLCJzdWIiOiJ1c2VyOjNlMjM3ZmVmLTBmYzEtNDFkNy1hMWRkLTU4MDY0NWNjNjJhMCIsImV4cCI6MTc2ODU4NDY4MywibmJmIjoxNzY4NTgzNzgzLCJpYXQiOjE3Njg1ODM3ODN9.D5WkHZBzJ3SLqxNhTPwFMvPQwZjLb5xJ0JvH9sO8cJg', // mulyadipermadi21
];

export const options = {
  scenarios: {
    // Direct 25k RPS untuk /api/users/me
    constant_25k: {
      executor: 'constant-arrival-rate',
      rate: 25000,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 1500,
      maxVUs: 3000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<800', 'p(99)<1500'],
    http_req_failed: ['rate<0.02'],
  },
};

const BASE_URL = 'http://localhost:8081';

export default function () {
  // Random pilih salah satu dari 4 token
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
    'response time < 2s': (r) => r.timings.duration < 2000,
  });

  // No sleep - maximum pressure
}

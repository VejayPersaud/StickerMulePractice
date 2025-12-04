import http from 'k6/http';
import { check, sleep } from 'k6';

//Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 20 },  //Ramp up to 20 VUs over 30s
    { duration: '1m', target: 20 },   //Stay at 20 VUs for 1 minute
    { duration: '30s', target: 0 },   //Ramp down to 0 VUs
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'], //95% of requests must complete in 200ms
    http_req_failed: ['rate<0.01'],   //Error rate must be below 1%
  },
};

//Base URL - change this to your Cloud Run URL
const BASE_URL = 'https://stickermule-app-386055911814.us-central1.run.app';

export default function () {
  //Make request to /health endpoint
  const res = http.get(`${BASE_URL}/health`);
  
  //Verify response
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response is OK': (r) => r.body === 'OK',
  });
  
  //Think time - simulate real user pausing between requests
  sleep(1);
}
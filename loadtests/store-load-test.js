import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

//Custom metrics
const cacheHitRate = new Rate('cache_hits');

//Test configuration - more aggressive load
export const options = {
  stages: [
    { duration: '30s', target: 50 },  //Ramp up to 50 VUs
    { duration: '2m', target: 50 },   //Sustain 50 VUs for 2 minutes
    { duration: '30s', target: 100 }, //Spike to 100 VUs
    { duration: '1m', target: 100 },  //Sustain spike
    { duration: '30s', target: 0 },   //Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  //Relaxed threshold (includes DB queries)
    http_req_failed: ['rate<0.35'],    
    cache_hits: ['rate>0.65'],         //Realistic under load
  },
};

const BASE_URL = 'https://stickermule-app-386055911814.us-central1.run.app';

//Store IDs to test (mix of valid and invalid)
const STORE_IDS = [1, 2, 9999]; //9999 will 404, tests error handling

export default function () {
  //Pick a random store ID
  const storeId = STORE_IDS[Math.floor(Math.random() * STORE_IDS.length)];
  
  //Make request
  const res = http.get(`${BASE_URL}/store?id=${storeId}`);
  
  //Track cache hits
  const cacheHeader = res.headers['X-Cache'];
  if (cacheHeader === 'HIT') {
    cacheHitRate.add(1);
  } else {
    cacheHitRate.add(0);
  }
  
  //Verify response
  check(res, {
    'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    'has expected response': (r) => r.status === 200 ? r.headers['X-Cache'] !== undefined : true,
  });
  
  //Shorter think time for store queries
  sleep(0.5);
}
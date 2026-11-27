// k6 load test for the anisette gateway. See docs/SELF_HOSTING.md → "Load testing".
//
// Only client_info and the WebSocket upgrade are exercised: neither makes
// anisette contact Apple. Never load-test get_headers or a full provisioning
// session against a real anisette server.
//
//   k6 run -e BASE=https://orbit.example.com -e TOKEN=… [-e RATE=50] gateway.js
import http from 'k6/http';
import ws from 'k6/ws';
import { check } from 'k6';

const BASE = __ENV.BASE;
const TOKEN = __ENV.TOKEN;
const RATE = Number(__ENV.RATE || 50);
const auth = { Authorization: `Bearer ${TOKEN}` };

export const options = {
  insecureSkipTLSVerify: __ENV.INSECURE === '1',
  scenarios: {
    // Signed-in requests, ramping up to RATE per second.
    client_info: {
      executor: 'ramping-arrival-rate',
      exec: 'clientInfo',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 500,
      stages: [
        { duration: '20s', target: RATE },
        { duration: '30s', target: RATE },
      ],
    },
    // Unauthenticated noise must stay cheap and refused.
    no_token: { executor: 'constant-arrival-rate', exec: 'noToken', rate: 10, timeUnit: '1s', duration: '50s', preAllocatedVUs: 10 },
    // Open WebSocket sessions and leave without sending anything.
    sessions: { executor: 'constant-arrival-rate', exec: 'session', rate: 2, timeUnit: '1s', duration: '50s', preAllocatedVUs: 20 },
  },
  thresholds: {
    'http_req_duration{kind:client_info}': ['p(95)<500'],
    checks: ['rate>0.99'],
  },
};

export function clientInfo() {
  const res = http.get(`${BASE}/anisette/v3/client_info`, { headers: auth, tags: { kind: 'client_info' } });
  check(res, { 'client_info 200': (r) => r.status === 200 });
}

export function noToken() {
  const res = http.get(`${BASE}/anisette/v3/client_info`, { tags: { kind: 'no_token' } });
  check(res, { 'refused without a token': (r) => r.status === 401 || r.status === 429 });
}

export function session() {
  const url = `${BASE.replace(/^http/, 'ws')}/anisette/v3/provisioning_session`;
  const res = ws.connect(url, { headers: auth }, (socket) => {
    socket.on('message', () => socket.close());
    socket.setTimeout(() => socket.close(), 3000);
  });
  check(res, { 'session upgraded': (r) => r && r.status === 101 });
}

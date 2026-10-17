import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// v1.3 Tier F2 — local-dev load baseline.
// Targets: POST /auth/login (once, in setup), then per-VU loop of
// POST /files/upload -> GET /files/download/<path> -> DELETE /files/<path>.
//
// Pace math (see docs/audit/v1.3-tier-F2-k6-harness.md): 4 VUs, one
// 3-request iteration per ~10 s per VU ≈ 72 req/min aggregate, under the
// default global sliding-window limiter (100 req/min). Any 429 means the
// pace math or limiter config drifted — the run fails loudly by design.
//
// Env: BASE_URL (default http://localhost:8080 — the code default when
// SERVER_PORT is unset; QUICKSTART's :3000 assumes an env override),
// K6_EMAIL / K6_PASSWORD (default admin@example.com / admin123 from `make seed`).

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const EMAIL = __ENV.K6_EMAIL || 'admin@example.com';
const PASSWORD = __ENV.K6_PASSWORD || 'admin123';
// 10 KiB fixture whose first 8 bytes are the PNG signature. The server
// allowlists by sniffed content type (http.DetectContentType), so the
// payload must BE a PNG/GIF/JPEG/WEBP/PDF — a text fixture gets 415.
const PAYLOAD = open('./payload-10k.png', 'b');

const loginDuration = new Trend('login_duration', true);

export const options = {
  stages: [
    { duration: '20s', target: 4 },
    { duration: '80s', target: 4 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(99)<1500'],
  },
};

// One iteration costs 3 requests. 4 VUs × 1 iter/10 s = 24 iter/min =
// 72 req/min aggregate, under the default global sliding-window limiter
// (100 req/min). Do NOT shorten this without raising RATE_LIMIT_MAX.
const PACE_SECONDS = 10;

export function setup() {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(res, { 'setup login 200': (r) => r.status === 200 });
  if (res.status !== 200) {
    throw new Error(`setup login failed: ${res.status} ${res.body}`);
  }
  const token = res.json('data.access_token');
  if (!token) {
    throw new Error(`setup login returned no access_token: ${res.body}`);
  }
  loginDuration.add(res.timings.duration);
  return { token };
}

export default function (data) {
  const auth = { Authorization: `Bearer ${data.token}` };

  // 1. Upload a small file into a dedicated directory.
  const form = {
    file: http.file(PAYLOAD, `k6-${__VU}-${Date.now()}.png`, 'image/png'),
    directory: 'k6-baseline',
  };
  const up = http.post(`${BASE_URL}/files/upload`, form, { headers: auth });
  check(up, { 'upload 201': (r) => r.status === 201 });
  // No early returns: every path below ends at the single paced sleep, so
  // a failing endpoint slows down instead of hot-spinning against the
  // limiter (a trip storm looks like 1500+ it/s of pure 429s).
  const path = up.status === 201 ? up.json('data.path') : null;

  if (path) {
    // 2. Download it back (mirrors GET /files/{id} read path).
    const down = http.get(`${BASE_URL}/files/download/${encodeURI(path)}`, {
      headers: auth,
    });
    check(down, { 'download 200': (r) => r.status === 200 });

    // 3. Delete it — the loop is self-cleaning, no residue in the dev DB.
    const del = http.del(`${BASE_URL}/files/${encodeURI(path)}`, null, {
      headers: auth,
    });
    check(del, { 'delete 204': (r) => r.status === 204 });
  }

  sleep(PACE_SECONDS);
}

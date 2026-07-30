// k6 stress test for the Virdan API (all services: auth, user, server, post,
// notification, chat, payment).
//
// Usage:
//   k6 run tests/k6/stress-test.js
//   k6 run -e BASE_URL=https://api.virdan.cloud/api -e PROFILE=stress tests/k6/stress-test.js
//
// The app is stateful (Postgres/Redis/MinIO backed) and most routes require a
// JWT, so this script provisions its own fixtures in setup():
//   - logs in a pool of pre-seeded users (see "Seeding test users" in the
//     accompanying README) instead of exercising the OTP-gated signup flow
//   - creates one shared "server", joins several seed users to it, and
//     creates a handful of posts to serve as read fixtures
//   - a DM conversation between two members for the chat endpoints
//
// Each VU iteration randomly picks one of several flows (weighted), so the
// full surface area of the API is exercised concurrently: reads, the full
// post lifecycle (create/like/save/comment/delete), chat, notifications, and
// auth (login/refresh/logout). Destructive/one-shot admin actions (kick,
// transfer ownership, delete server, delete account) are intentionally left
// out of the load generator since they would destroy the shared fixture the
// rest of the run depends on.

import http from 'k6/http';
import encoding from 'k6/encoding';
import { check, group, sleep, fail } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8081/api').replace(/\/$/, '');
const SEED_EMAIL_PREFIX = __ENV.SEED_EMAIL_PREFIX || 'k6user';
const SEED_EMAIL_DOMAIN = __ENV.SEED_EMAIL_DOMAIN || '@test.local';
const SEED_PASSWORD = __ENV.SEED_PASSWORD || 'K6StressTest!23';
const SEED_USER_COUNT = parseInt(__ENV.SEED_USER_COUNT || '30', 10);
// Users 1..MEMBER_COUNT are logged in once in setup() and shared (read-only,
// as far as auth goes) by every flow except auth_cycle. Users
// (MEMBER_COUNT+1)..SEED_USER_COUNT are reserved exclusively for auth_cycle.
// This split matters because login/refresh/logout on this API invalidate any
// previously issued access token for that same user (single active session
// per user) — sharing a user between the fixture pool and the login/logout
// churn would randomly 401 every other flow using that user.
const MEMBER_COUNT = Math.min(SEED_USER_COUNT - 5, parseInt(__ENV.MEMBER_COUNT || '15', 10));
const FIXTURE_POST_COUNT = parseInt(__ENV.FIXTURE_POST_COUNT || '15', 10);

// PROFILE picks a preset load shape; override VUS/DURATION directly for full control.
const PROFILE = __ENV.PROFILE || 'load'; // smoke | load | stress | soak
const PROFILES = {
  smoke: { vus: 2, duration: '20s' },
  load: { vus: 15, duration: '1m' },
  stress: { vus: 60, duration: '2m' },
  soak: { vus: 20, duration: '10m' },
};
const shape = PROFILES[PROFILE] || PROFILES.load;
const TARGET_VUS = parseInt(__ENV.VUS || shape.vus, 10);
const DURATION = __ENV.DURATION || shape.duration;

// A 1x1 transparent PNG, used as the upload payload for image endpoints.
const PIXEL_PNG = encoding.b64decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
  'std',
  'binary'
);

const errorRate = new Rate('virdan_errors');
const authDuration = new Trend('virdan_auth_duration');
const readDuration = new Trend('virdan_read_duration');
const writeDuration = new Trend('virdan_write_duration');

export const options = {
  scenarios: {
    browse: {
      executor: 'ramping-vus',
      exec: 'browseFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: TARGET_VUS },
        { duration: DURATION, target: TARGET_VUS },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    content_lifecycle: {
      executor: 'ramping-vus',
      exec: 'contentFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: Math.max(2, Math.round(TARGET_VUS * 0.4)) },
        { duration: DURATION, target: Math.max(2, Math.round(TARGET_VUS * 0.4)) },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    social_actions: {
      executor: 'ramping-vus',
      exec: 'socialFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: Math.max(2, Math.round(TARGET_VUS * 0.5)) },
        { duration: DURATION, target: Math.max(2, Math.round(TARGET_VUS * 0.5)) },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    chat: {
      executor: 'ramping-vus',
      exec: 'chatFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: Math.max(1, Math.round(TARGET_VUS * 0.25)) },
        { duration: DURATION, target: Math.max(1, Math.round(TARGET_VUS * 0.25)) },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    notifications: {
      executor: 'ramping-vus',
      exec: 'notifFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: Math.max(1, Math.round(TARGET_VUS * 0.25)) },
        { duration: DURATION, target: Math.max(1, Math.round(TARGET_VUS * 0.25)) },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    auth_cycle: {
      executor: 'ramping-vus',
      exec: 'authFlow',
      startVUs: 0,
      stages: [
        { duration: '20s', target: Math.max(1, Math.round(TARGET_VUS * 0.2)) },
        { duration: DURATION, target: Math.max(1, Math.round(TARGET_VUS * 0.2)) },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
    health: {
      executor: 'constant-arrival-rate',
      exec: 'healthFlow',
      rate: parseInt(__ENV.HEALTH_RPS || '5', 10),
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
  },
  thresholds: {
    virdan_errors: ['rate<0.05'],
    http_req_duration: ['p(95)<2000'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function authedJSON(token) {
  return { headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' } };
}

function authedOnly(token) {
  return { headers: { Authorization: `Bearer ${token}` } };
}

function recordErr(ok) {
  errorRate.add(!ok);
}

function randInt(max) {
  return Math.floor(Math.random() * max);
}

function pick(arr) {
  return arr[randInt(arr.length)];
}

// clientMessageId must be a UUID (server-side validation); k6's JS runtime
// has no crypto.randomUUID, so generate a good-enough v4-shaped one.
function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

function loginUser(index) {
  const email = `${SEED_EMAIL_PREFIX}${index}${SEED_EMAIL_DOMAIN}`;
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password: SEED_PASSWORD }),
    { headers: { 'Content-Type': 'application/json' }, tags: { name: 'auth_login' } }
  );
  authDuration.add(res.timings.duration);
  const ok = check(res, { 'login: 200': (r) => r.status === 200 });
  recordErr(ok);
  if (!ok) return null;
  const body = res.json();
  return { index, email, userId: null, token: body.accessToken, refreshToken: body.refreshToken };
}

// ---------------------------------------------------------------------------
// setup(): provision the shared fixture once before the load starts.
// ---------------------------------------------------------------------------

export function setup() {
  const health = http.get(`${BASE_URL}/health`);
  if (health.status !== 200) {
    fail(`API is not healthy at ${BASE_URL}/health (status ${health.status}). Aborting.`);
  }

  // Only log in the member pool (1..MEMBER_COUNT) here. Users beyond that are
  // reserved for auth_cycle's own login/refresh/logout churn — see the note
  // by MEMBER_COUNT above for why they must not overlap.
  const users = [];
  for (let i = 1; i <= MEMBER_COUNT; i++) {
    const u = loginUser(i);
    if (u) users.push(u);
  }
  if (users.length < 3) {
    fail(
      `Only ${users.length} seed users could log in. Seed at least ` +
        `SEED_USER_COUNT (${SEED_USER_COUNT}) users ` +
        `(${SEED_EMAIL_PREFIX}1${SEED_EMAIL_DOMAIN}..${SEED_EMAIL_PREFIX}N${SEED_EMAIL_DOMAIN}, ` +
        `password "${SEED_PASSWORD}") before running this script — see tests/k6/README.md.`
    );
  }

  // Resolve a real category id.
  const catRes = http.get(`${BASE_URL}/servers/categories`, authedOnly(users[0].token));
  const categories = catRes.status === 200 ? catRes.json('data') : [];
  const categoryId = categories.length > 0 ? categories[0].id : 1;

  // Create the shared server owned by users[0].
  const owner = users[0];
  const serverRes = http.post(
    `${BASE_URL}/servers/create`,
    {
      name: `K6 Stress Server ${Date.now()}`,
      shortName: `K6${Date.now()}`.slice(0, 10),
      description: 'Fixture server created by the k6 stress test',
      categoryId: String(categoryId),
      isPrivate: 'false',
      nickname: 'K6 Owner',
      username: `k6owner${Date.now()}`,
      // Presence of a file field is what makes k6 encode this request as
      // multipart/form-data (required by the API) instead of urlencoded.
      serverAvatar: http.file(PIXEL_PNG, 'avatar.png', 'image/png'),
    },
    authedOnly(owner.token)
  );
  if (serverRes.status !== 200) {
    fail(`Failed to create fixture server: ${serverRes.status} ${serverRes.body}`);
  }
  const serverId = serverRes.json('server.id');

  // Join a slice of the pool into the server.
  const members = [owner];
  for (let i = 1; i < MEMBER_COUNT; i++) {
    const u = users[i];
    const joinRes = http.post(
      `${BASE_URL}/servers/${serverId}/join`,
      {
        nickname: `Member ${i}`,
        username: `k6member${i}${Date.now()}`,
        // JoinServer has no optional file field of its own; attach a throwaway
        // one so k6 encodes the request as multipart/form-data, which the
        // handler requires (it ignores unknown form fields).
        _multipart: http.file(PIXEL_PNG, 'x.png', 'image/png'),
      },
      authedOnly(u.token)
    );
    if (joinRes.status === 200) members.push(u);
  }

  // Create an invite link (exercised by browseFlow) and a fixture set of posts.
  const inviteRes = http.post(
    `${BASE_URL}/servers/${serverId}/invites`,
    JSON.stringify({ maxUses: 0 }),
    authedJSON(owner.token)
  );
  const inviteCode = inviteRes.status === 200 ? inviteRes.json('inviteCode') || inviteRes.json('code') : null;

  const postIds = [];
  for (let i = 0; i < FIXTURE_POST_COUNT; i++) {
    const author = pick(members);
    const postRes = http.post(
      `${BASE_URL}/servers/${serverId}/posts`,
      {
        caption: `Fixture post #${i} for k6 stress testing`,
        image: http.file(PIXEL_PNG, `fixture-${i}.png`, 'image/png'),
      },
      authedOnly(author.token)
    );
    if (postRes.status === 200) postIds.push(postRes.json('id'));
  }

  // Seed a DM conversation between the first two members for chatFlow.
  let conversationId = null;
  if (members.length >= 2) {
    const meRes = http.get(`${BASE_URL}/users/me`, authedOnly(members[1].token));
    const peerUserId = meRes.status === 200 ? meRes.json('id') || meRes.json('userId') : null;
    if (peerUserId) {
      const convRes = http.post(
        `${BASE_URL}/servers/${serverId}/conversations`,
        JSON.stringify({ peerUserId }),
        authedJSON(members[0].token)
      );
      if (convRes.status === 200) conversationId = convRes.json('id');
    }
  }

  return {
    baseUrl: BASE_URL,
    members,
    serverId,
    categoryId,
    inviteCode,
    postIds,
    conversationId,
  };
}

// ---------------------------------------------------------------------------
// Flows (one per scenario)
// ---------------------------------------------------------------------------

// Read-heavy browsing across nearly every GET endpoint.
export function browseFlow(data) {
  const user = pick(data.members);
  const h = authedOnly(user.token);

  group('browse', () => {
    let r = http.get(`${BASE_URL}/users/me`, h);
    recordErr(check(r, { 'me: 200': (x) => x.status === 200 }));
    readDuration.add(r.timings.duration);

    r = http.get(`${BASE_URL}/servers/categories`, h);
    recordErr(check(r, { 'categories: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers?limit=20`, h);
    recordErr(check(r, { 'discovery: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/me`, h);
    recordErr(check(r, { 'my servers: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}`, h);
    recordErr(check(r, { 'server by id: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/members?limit=20`, h);
    recordErr(check(r, { 'members: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/members/me`, h);
    recordErr(check(r, { 'my role: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/profile/me`, h);
    recordErr(check(r, { 'my profile: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/profiles/history`, h);
    recordErr(check(r, { 'profile history: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/posts?limit=10`, h);
    recordErr(check(r, { 'server posts: 200': (x) => x.status === 200 }));
    readDuration.add(r.timings.duration);

    r = http.get(`${BASE_URL}/servers/${data.serverId}/posts/me?limit=10`, h);
    recordErr(check(r, { 'my posts: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/posts/saved?limit=10`, h);
    recordErr(check(r, { 'saved posts: 200': (x) => x.status === 200 }));

    r = http.get(
      `${BASE_URL}/servers/${data.serverId}/posts/search?q=k6`,
      h
    );
    recordErr(check(r, { 'search posts: 200': (x) => x.status === 200 }));

    if (data.postIds.length > 0) {
      const postId = pick(data.postIds);
      r = http.get(`${BASE_URL}/posts/${postId}`, h);
      recordErr(check(r, { 'get post: 200': (x) => x.status === 200 }));

      r = http.get(`${BASE_URL}/posts/${postId}/comments?limit=10`, h);
      recordErr(check(r, { 'get comments: 200': (x) => x.status === 200 }));
    }

    r = http.get(`${BASE_URL}/servers/${data.serverId}/notifications?limit=10`, h);
    recordErr(check(r, { 'notif feed: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/notifications/unread-count`, h);
    recordErr(check(r, { 'unread count: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/plus`, h);
    recordErr(check(r, { 'plus status: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/me/plus-orders?limit=10`, h);
    recordErr(check(r, { 'plus orders: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/members/dm?limit=10`, h);
    recordErr(check(r, { 'dm members: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/conversations?limit=10`, h);
    recordErr(check(r, { 'conversations: 200': (x) => x.status === 200 }));

    if (data.inviteCode) {
      r = http.get(`${BASE_URL}/servers/invites/${data.inviteCode}`, h);
      recordErr(check(r, { 'invite info: 200': (x) => x.status === 200 }));
    }
  });

  sleep(Math.random() * 1.5);
}

// Full create -> engage -> delete lifecycle for a post; exercises every post
// write endpoint plus the multipart image upload path.
export function contentFlow(data) {
  const author = pick(data.members);
  const h = authedOnly(author.token);

  group('content_lifecycle', () => {
    let r = http.post(
      `${BASE_URL}/servers/${data.serverId}/posts`,
      {
        caption: `k6 lifecycle post ${Date.now()}-${randInt(100000)}`,
        image: http.file(PIXEL_PNG, `k6-${Date.now()}.png`, 'image/png'),
      },
      h
    );
    writeDuration.add(r.timings.duration);
    const created = check(r, { 'create post: 200': (x) => x.status === 200 });
    recordErr(created);
    if (!created) return;
    const postId = r.json('id');

    r = http.put(
      `${BASE_URL}/servers/${data.serverId}/posts/${postId}`,
      JSON.stringify({ caption: `edited by k6 ${Date.now()}` }),
      { headers: { Authorization: `Bearer ${author.token}`, 'Content-Type': 'application/json' } }
    );
    recordErr(check(r, { 'update post: 200': (x) => x.status === 200 }));

    r = http.post(`${BASE_URL}/posts/${postId}/likes`, null, h);
    recordErr(check(r, { 'like post: 2xx/409': (x) => x.status < 500 }));

    r = http.post(`${BASE_URL}/posts/${postId}/saves`, null, h);
    recordErr(check(r, { 'save post: 2xx/409': (x) => x.status < 500 }));

    r = http.post(
      `${BASE_URL}/posts/${postId}/comments`,
      JSON.stringify({ content: `nice post! from k6 ${randInt(100000)}` }),
      { headers: { Authorization: `Bearer ${author.token}`, 'Content-Type': 'application/json' } }
    );
    recordErr(check(r, { 'create comment: 200': (x) => x.status === 200 }));
    const commentId = r.status === 200 ? r.json('id') : null;

    r = http.get(`${BASE_URL}/posts/${postId}/comments?limit=10`, h);
    recordErr(check(r, { 'list comments: 200': (x) => x.status === 200 }));

    r = http.del(`${BASE_URL}/posts/${postId}/likes`, null, h);
    recordErr(check(r, { 'unlike post: 2xx/404': (x) => x.status < 500 }));

    r = http.del(`${BASE_URL}/posts/${postId}/saves`, null, h);
    recordErr(check(r, { 'unsave post: 2xx/404': (x) => x.status < 500 }));

    if (commentId) {
      r = http.del(`${BASE_URL}/posts/${postId}/comments/${commentId}`, null, h);
      recordErr(check(r, { 'delete comment: 2xx': (x) => x.status < 500 }));
    }

    r = http.del(`${BASE_URL}/servers/${data.serverId}/posts/${postId}`, null, h);
    recordErr(check(r, { 'delete post: 2xx': (x) => x.status < 500 }));
    writeDuration.add(r.timings.duration);
  });

  sleep(Math.random() * 2);
}

// Concurrent like/save/comment churn against the shared read fixture posts.
export function socialFlow(data) {
  if (data.postIds.length === 0) return;
  const user = pick(data.members);
  const postId = pick(data.postIds);
  const h = authedOnly(user.token);

  group('social_actions', () => {
    let r = http.post(`${BASE_URL}/posts/${postId}/likes`, null, h);
    recordErr(check(r, { 'like: 2xx/409': (x) => x.status < 500 }));

    r = http.post(`${BASE_URL}/posts/${postId}/saves`, null, h);
    recordErr(check(r, { 'save: 2xx/409': (x) => x.status < 500 }));

    r = http.post(
      `${BASE_URL}/posts/${postId}/comments`,
      JSON.stringify({ content: `+1 from k6 ${randInt(100000)}` }),
      { headers: { Authorization: `Bearer ${user.token}`, 'Content-Type': 'application/json' } }
    );
    recordErr(check(r, { 'comment: 200': (x) => x.status === 200 }));

    if (Math.random() < 0.5) {
      r = http.del(`${BASE_URL}/posts/${postId}/likes`, null, h);
      recordErr(check(r, { 'unlike: 2xx/404': (x) => x.status < 500 }));
    }
    if (Math.random() < 0.5) {
      r = http.del(`${BASE_URL}/posts/${postId}/saves`, null, h);
      recordErr(check(r, { 'unsave: 2xx/404': (x) => x.status < 500 }));
    }
  });

  sleep(Math.random());
}

// DM chat: get-or-create conversation, send a message, list, mark read.
export function chatFlow(data) {
  if (data.members.length < 2) return;
  const sender = pick(data.members);
  let peer = pick(data.members);
  if (peer.index === sender.index) peer = data.members[(data.members.indexOf(peer) + 1) % data.members.length];

  group('chat', () => {
    const meRes = http.get(`${BASE_URL}/users/me`, authedOnly(peer.token));
    const peerUserId = meRes.status === 200 ? meRes.json('id') || meRes.json('userId') : null;
    if (!peerUserId) return;

    let r = http.post(
      `${BASE_URL}/servers/${data.serverId}/conversations`,
      JSON.stringify({ peerUserId }),
      authedJSON(sender.token)
    );
    recordErr(check(r, { 'get/create conversation: 200': (x) => x.status === 200 }));
    if (r.status !== 200) return;
    const conversationId = r.json('id');

    r = http.post(
      `${BASE_URL}/conversations/${conversationId}/messages`,
      JSON.stringify({ content: `hey from k6 ${Date.now()}`, clientMessageId: uuidv4() }),
      authedJSON(sender.token)
    );
    recordErr(check(r, { 'send message: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/conversations/${conversationId}/messages?limit=20`, authedOnly(sender.token));
    recordErr(check(r, { 'list messages: 200': (x) => x.status === 200 }));

    r = http.post(`${BASE_URL}/conversations/${conversationId}/read`, JSON.stringify({}), authedJSON(peer.token));
    recordErr(check(r, { 'mark read: 2xx': (x) => x.status < 500 }));
  });

  sleep(Math.random());
}

// Device registration + notification preferences + feed interactions.
export function notifFlow(data) {
  const user = pick(data.members);
  const h = authedOnly(user.token);
  const token = `k6-device-${__VU}-${__ITER}-${Date.now()}`;

  group('notifications', () => {
    let r = http.post(
      `${BASE_URL}/devices`,
      JSON.stringify({ token, platform: 'android' }),
      authedJSON(user.token)
    );
    recordErr(check(r, { 'register device: 2xx': (x) => x.status < 500 }));

    r = http.put(
      `${BASE_URL}/users/me/notification-preferences`,
      JSON.stringify({ notifLike: true, notifComment: true, notifReply: true }),
      authedJSON(user.token)
    );
    recordErr(check(r, { 'update notif prefs: 200': (x) => x.status === 200 }));

    r = http.get(`${BASE_URL}/servers/${data.serverId}/notifications?limit=10`, h);
    recordErr(check(r, { 'notif feed: 200': (x) => x.status === 200 }));
    const items = r.status === 200 ? r.json('data') || [] : [];
    if (items.length > 0) {
      const notifId = items[0].id;
      r = http.post(`${BASE_URL}/servers/${data.serverId}/notifications/${notifId}/read`, null, h);
      recordErr(check(r, { 'mark notif read: 2xx': (x) => x.status < 500 }));
    }

    r = http.del(
      `${BASE_URL}/devices`,
      JSON.stringify({ token }),
      authedJSON(user.token)
    );
    recordErr(check(r, { 'unregister device: 2xx': (x) => x.status < 500 }));
  });

  sleep(Math.random());
}

// Self-contained login -> refresh -> logout cycle, independent of the shared
// fixture so token rotation from one VU never races another.
export function authFlow() {
  group('auth_cycle', () => {
    // Use only the reserved auth-pool range (MEMBER_COUNT+1..SEED_USER_COUNT)
    // — see the note by MEMBER_COUNT: logging in/out here invalidates that
    // user's session everywhere, so it must never collide with data.members.
    const idx = MEMBER_COUNT + 1 + randInt(Math.max(1, SEED_USER_COUNT - MEMBER_COUNT));
    const user = loginUser(idx);
    if (!user) return;

    let r = http.post(
      `${BASE_URL}/auth/refresh`,
      JSON.stringify({ refreshToken: user.refreshToken }),
      { headers: { 'Content-Type': 'application/json' }, tags: { name: 'auth_refresh' } }
    );
    authDuration.add(r.timings.duration);
    const refreshed = check(r, { 'refresh: 200': (x) => x.status === 200 });
    recordErr(refreshed);
    const accessToken = refreshed ? r.json('accessToken') : user.token;

    r = http.post(`${BASE_URL}/auth/logout`, null, {
      headers: { Authorization: `Bearer ${accessToken}` },
      tags: { name: 'auth_logout' },
    });
    recordErr(check(r, { 'logout: 2xx': (x) => x.status < 500 }));
  });

  sleep(Math.random() * 2);
}

export function healthFlow() {
  const r = http.get(`${BASE_URL}/health`, { tags: { name: 'health' } });
  recordErr(check(r, { 'health: 200': (x) => x.status === 200 }));
}

// ---------------------------------------------------------------------------
// teardown(): best-effort cleanup of the shared fixture (posts are also
// individually cleaned up by contentFlow; the fixture server/posts created
// here are left for inspection by default — set CLEANUP=1 to remove them).
// ---------------------------------------------------------------------------

export function teardown(data) {
  if (__ENV.CLEANUP !== '1') return;
  const owner = data.members[0];
  for (const postId of data.postIds) {
    http.del(`${BASE_URL}/servers/${data.serverId}/posts/${postId}`, null, authedOnly(owner.token));
  }
  http.del(`${BASE_URL}/servers/${data.serverId}`, null, authedOnly(owner.token));
}

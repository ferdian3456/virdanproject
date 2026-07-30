# k6 stress test

`stress-test.js` load-tests essentially the whole Virdan API surface across
every service (auth, user, server, post, notification, chat, payment) in one
run, using several concurrent k6 scenarios:

| Scenario            | What it does                                                                 |
|----------------------|-------------------------------------------------------------------------------|
| `health`             | Hammers `GET /health` at a fixed rate.                                       |
| `browse`             | Read-only tour of ~20 GET endpoints (servers, posts, members, notifications, chat, payment status, invites). |
| `content_lifecycle`  | Full post lifecycle: create (multipart image upload) → update caption → like → save → comment → list comments → unlike → unsave → delete comment → delete post. |
| `social_actions`     | Concurrent like/save/comment churn against a shared pool of fixture posts.   |
| `chat`               | Get-or-create DM conversation → send message → list messages → mark read.    |
| `notifications`      | Register device → update notification prefs → read feed → mark read → unregister device. |
| `auth_cycle`         | Fresh login → refresh → logout, using a dedicated pool of users (see below). |

Destructive/admin-only actions (kick member, transfer ownership, delete
server, delete account, assign role) are **not** in the load generator on
purpose — they'd tear down the shared fixture the rest of the run depends on.
Payment checkout is also excluded since it calls the real Xendit API.

## Why it needs seeded users instead of signing up fresh ones

Signup (`/auth/signup/*`) is OTP-gated over email, so it needs a working SMTP
server to complete — not something a load generator should depend on. Instead,
the script logs in with a pool of users inserted directly into Postgres.

**One quirk worth knowing:** this API keeps a *single active access token per
user* (login/refresh overwrite it, logout clears it). So the script splits its
user pool in two non-overlapping ranges:
- users `1..MEMBER_COUNT` are logged in once in `setup()` and shared read-only
  (as far as auth goes) by every scenario except `auth_cycle`.
- users `MEMBER_COUNT+1..SEED_USER_COUNT` are reserved exclusively for
  `auth_cycle`'s own login/refresh/logout churn.

Never point `auth_cycle` and the rest of the pool at the same users — every
login/logout would invalidate the other scenarios' tokens and you'll see a
wave of spurious 401s.

## Setup

1. Seed the test users (bcrypt hash for `K6StressTest!23` is already baked
   into the script):

   ```bash
   psql "$POSTGRES_URL" -f tests/k6/seed-users.sql
   ```

2. Run the app against a real Postgres/Redis/MinIO stack (see the repo
   README's "Running Locally" section).

3. Install k6 (https://k6.io/docs/get-started/installation/), or build it
   from source if `dl.k6.io` isn't reachable from your network:

   ```bash
   go install go.k6.io/k6@latest
   ```

## Running

```bash
# quick sanity check (2 VUs, 20s)
k6 run -e PROFILE=smoke tests/k6/stress-test.js

# default load test (15 VUs, 1m)
k6 run tests/k6/stress-test.js

# heavier stress test (60 VUs, 2m)
k6 run -e PROFILE=stress tests/k6/stress-test.js

# soak test (20 VUs, 10m) — watch for leaks/degradation over time
k6 run -e PROFILE=soak tests/k6/stress-test.js

# against a non-default host
k6 run -e BASE_URL=https://api.virdan.cloud/api -e PROFILE=load tests/k6/stress-test.js
```

### Useful env vars

| Var | Default | Meaning |
|---|---|---|
| `BASE_URL` | `http://localhost:8081/api` | API base URL. |
| `PROFILE` | `load` | `smoke` \| `load` \| `stress` \| `soak` — picks VUs/duration. |
| `VUS` / `DURATION` | profile-dependent | Override the profile's peak VUs / hold duration directly. |
| `SEED_USER_COUNT` | `30` | Total seeded users available (must match how many you seeded). |
| `MEMBER_COUNT` | `15` | How many of them join the shared fixture server; the rest are reserved for `auth_cycle`. |
| `SEED_PASSWORD` | `K6StressTest!23` | Must match the password hashed into `seed-users.sql`. |
| `FIXTURE_POST_COUNT` | `15` | Read-fixture posts created in `setup()` for `browse`/`social_actions` to hit. |
| `HEALTH_RPS` | `5` | Request rate for the dedicated health-check scenario. |
| `CLEANUP` | unset | Set to `1` to delete the fixture server/posts in `teardown()` after the run. |

Results print to stdout; add `--out json=results.json` (or any other k6
output plugin) to capture raw metrics.

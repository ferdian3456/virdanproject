-- Seeds throwaway users for tests/k6/stress-test.js, bypassing the OTP-gated
-- signup flow (which needs a working SMTP server to complete).
--
-- The k6 script logs in with these directly via POST /auth/login, so it never
-- touches /auth/signup/*.
--
-- Usage:
--   psql "$POSTGRES_URL" -v password_hash="'<bcrypt-hash>'" -f tests/k6/seed-users.sql
--
-- Generate the bcrypt hash for SEED_PASSWORD (default below) with any bcrypt
-- tool, e.g. from the repo root:
--   cat <<'EOF' > /tmp/bcryptgen.go
--   package main
--   import ("fmt"; "os"; "golang.org/x/crypto/bcrypt")
--   func main() {
--       h, _ := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
--       fmt.Println(string(h))
--   }
--   EOF
--   go run /tmp/bcryptgen.go 'K6StressTest!23'
--
-- Then pass that hash in as :'password_hash', or just edit the DEFAULT below.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
DECLARE
  i int;
  uid uuid;
  seed_count int := 30; -- keep in sync with SEED_USER_COUNT in stress-test.js
  -- bcrypt hash of "K6StressTest!23" — regenerate if you change SEED_PASSWORD.
  pw_hash text := '$2a$10$bAneysEnmdrzLVpgbBMYJOrYtzhCE35Sr5wm1O4CY.V9aHztDE8a.';
BEGIN
  FOR i IN 1..seed_count LOOP
    uid := gen_random_uuid();
    INSERT INTO users (id, email, password, settings, created_at, updated_at, created_by, updated_by)
    VALUES (
      uid,
      'k6user' || i || '@test.local',
      pw_hash,
      '{}',
      now(), now(), uid, uid
    )
    ON CONFLICT DO NOTHING;
  END LOOP;
END $$;

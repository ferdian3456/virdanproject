## Refresh Token Flow

### Overview
Uses a valid refresh token to obtain a new access token without requiring the user to log in again. Implements token rotation for security - each refresh invalidates all tokens from the same token family.

### Auth
No authentication required.

### Business Logic
1. Validate request payload — `refreshToken` is required
2. Hash the refresh token with SHA256 to look up in database
3. Look up refresh token by hash in database — return error if not found
4. Check if token is revoked (`revoked_at IS NULL`) — if revoked:
   - **SECURITY ESCALATION**: Revoke ALL refresh tokens for this user (possible token theft)
   - Return unauthorized error with message "Session expired. Please login again."
5. Check if token is expired (`expires_at > NOW()`) — return unauthorized error if expired
6. Begin database transaction
7. Revoke all refresh tokens with the same `token_family` (token rotation)
8. Generate new JWT access token (15 minutes expiry)
9. Generate new refresh token (UUID) with new `token_family`
10. Hash new refresh token and store to database
11. Commit transaction
12. Return new token pair to client

### Database Operations

#### PostgreSQL — Get Refresh Token by Hash
```sql
SELECT id, user_id, token_hash, token_family, expires_at, revoked_at, created_at, updated_at, created_by, updated_by
FROM refresh_tokens
WHERE token_hash = $1
LIMIT 1
```
- **Table**: `refresh_tokens`
- **Columns read**: all columns except `id` for validation
- **Filter**: `token_hash` (SHA-256 hash of refresh token from client)

#### PostgreSQL — Revoke Tokens by Family
```sql
UPDATE refresh_tokens
SET revoked_at = $1, updated_at = $2, updated_by = $3
WHERE user_id = $4 AND token_family = $5 AND revoked_at IS NULL
```
- **Table**: `refresh_tokens`
- **Columns updated**: `revoked_at`, `updated_at`, `updated_by`
- **Filter**: `user_id`, `token_family`, `revoked_at IS NULL` (only active tokens)

#### PostgreSQL — Create New Refresh Token
```sql
INSERT INTO refresh_tokens
(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
```
- **Table**: `refresh_tokens`
- **Columns**: all columns
- **Note**: `created_by` and `updated_by` use the same value (user ID)

### Error Cases
- Missing/invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Refresh token empty → `400` with "Refresh token is required to not be empty"
- Refresh token not found in database → `401` with "Refresh token is not found"
- Refresh token already revoked → `401` with "Session expired. Please login again." + **ALL user tokens revoked**
- Refresh token expired → `401` with "Refresh token is expired"
- Transaction commit failed → `500` with internal server error

### Flow
```
Request → Validate Payload → Hash Token → Lookup Token (DB) → Check Revoked → Check Expired → Begin TX → Revoke Family (DB) → Generate Access Token → Generate Refresh Token → Store New Token (DB) → Commit TX → Response
```

### Token Rotation (Security)

Token rotation prevents token theft by ensuring each refresh token can only be used once:

```
Initial Login:
→ Access Token-A (15 min)
→ Refresh Token-A (family: "abc-123", 7 days)

Refresh (client uses Refresh Token-A):
→ Revoke all tokens with family "abc-123"
→ Access Token-B (15 min)
→ Refresh Token-B (family: "xyz-789", 7 days)

If attacker stole Refresh Token-A:
→ Attacker tries to use Refresh Token-A
→ Server: "Token has been revoked" ❌
→ User can still use Refresh Token-B ✅
```

### Security Escalation (Token Theft Detection)

When a revoked token is detected, the system assumes possible token theft:

```
Normal Flow:
→ User login → Token-A
→ Attacker steals Token-A
→ Attacker refreshes → Token-B, Token-A revoked
→ User tries to refresh with Token-A (revoked)
→ Server: DETECTS THEFT! Revokes ALL tokens (including Token-B)
→ Both user and attacker must login again
```

This prevents attackers from maintaining access after token theft is detected.

### Token Info
- **Access Token** — JWT, short-lived (15 minutes), used for API authentication
- **Refresh Token** — UUID, long-lived (7 days), used to get new access tokens
- **Token Family** — UUID group identifier, all tokens in same family are revoked on refresh
- **Token Type** — Bearer

### Durations
- **Access Token**: 15 minutes (900 seconds)
- **Refresh Token**: 7 days (604800 seconds)

### Security Notes
- Refresh tokens are stored as SHA-256 hashes in the database
- Original refresh token string is never stored, only the hash
- Token rotation ensures stolen tokens become useless after first use
- **Security Escalation**: Attempt to use revoked token triggers full token revocation
- This detects token theft and forces re-authentication from all devices
- Transaction ensures atomicity — either all operations succeed or all fail

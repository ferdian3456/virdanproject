## Overview

This API starts a Virdan Plus purchase for a server: it creates a `PENDING` order row and opens a hosted payment session with Xendit, returning a payment link the client should redirect the user to. The order is only marked `PAID` later, asynchronously, when Xendit calls the webhook (see `xendit_webhook.md`) after the user actually completes payment.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant Xendit

    Client->>BE: POST /api/servers/(serverId)/plus/checkout
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: COUNT server_members WHERE server_id = $1 AND user_id = $2
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: Check for an active (unexpired PAID) order
    alt Already active
        BE-->>Client: 409 Server already has an active Virdan Plus
    end
    BE->>Postgres: INSERT INTO server_plus_orders status='PENDING'
    BE->>Xendit: POST /sessions (PAYMENT_LINK, amount, currency IDR, success/cancel URLs)
    alt Xendit request fails
        BE-->>Client: 500 Internal Server Error
    end
    BE->>Postgres: UPDATE server_plus_orders SET xendit_session_id (best-effort, failure is non-fatal)
    BE-->>Client: 200 PlusCheckoutResponse {orderId, paymentUrl}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                | Column                                                     | Action | Notes                                                       |
| --------------------- | ----------------------------------------------------------- | ------ | -------------------------------------------------------------- |
| `server_members`      | (count)                                                       | SELECT | Check membership                                                |
| `server_plus_orders`  | plus_expires_at                                               | SELECT | Reject if server already has an active subscription             |
| `server_plus_orders`  | id, server_id, user_id, reference_id, base/tax/total_idr, status | INSERT | New order, `status='PENDING'`, `reference_id = "virdan-plus-{orderId}"` |
| `server_plus_orders`  | xendit_session_id                                             | UPDATE | Attach the Xendit session id once created (best-effort; a failure here does not fail the request) |

---

## Notes External API (Xendit)

- `POST {XENDIT_API_BASE_URL}/sessions`, Basic Auth with `XENDIT_SECRET_KEY`.
- Body: `session_type: "PAY"`, `mode: "PAYMENT_LINK"`, `amount`, `currency: "IDR"`, `country: "ID"`, `success_return_url`, `cancel_return_url`, `description: "Virdan Plus (30 days)"`.
- Response used: `payment_session_id` (stored as `xendit_session_id`) and `payment_link_url` (returned to the client as `paymentUrl`).

---

## Prerequisites

Caller must be a member of the target server, and the server must not already have an active Virdan Plus subscription.

---

## Request Validation

Path parameter:

| Field      | Type   | Required | Rules           |
| ---------- | ------ | -------- | --------------- |
| `serverId` | string | yes      | Required, UUID  |

No body.

---

## Response

### 200 OK

```json
{
  "orderId": "order-uuid",
  "paymentUrl": "https://checkout.xendit.co/..."
}
```

| Field        | Type   | Description                                  |
| ------------ | ------ | --------------------------------------------- |
| `orderId`    | string | Newly created order UUID                       |
| `paymentUrl` | string | Xendit hosted payment link to redirect the user to |

### 400 Bad Request

| `error_message`                | Cause        |
| ------------------------------- | ------------ |
| `serverId is not a valid UUID`  | Invalid UUID |

### 403 Forbidden

| `error_message`                        | Cause        |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Not a member |

### 409 Conflict

| `error_message`                              | Cause                                       |
| ----------------------------------------------- | ---------------------------------------------- |
| `Server already has an active Virdan Plus`      | The server already has an unexpired `PAID` order |

### 401 Unauthorized

Standard auth errors.

### 500 Internal Server Error

Returned if the call to Xendit's session API fails (network error, non-2xx response, or a malformed response missing `payment_link_url`). The `PENDING` order row created just before this call is **not** rolled back — a stuck `PENDING` order with no session id will simply never transition to `PAID` since no webhook will ever reference it.

---

## Update

This documentation was last updated on 20 July 2026.

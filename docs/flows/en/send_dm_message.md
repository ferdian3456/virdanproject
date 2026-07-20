## Overview

This API sends a direct message in an existing conversation. It is idempotent per `(conversationId, senderId, clientMessageId)` — retrying the same send (e.g. after a client-side timeout) with the same `clientMessageId` returns the original message instead of creating a duplicate. On a genuinely new message it also fans the message out to the peer over their live WebSocket connection (if any, see `websocket_realtime.md`) and triggers a best-effort push notification.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant WS Hub
    participant FCM

    Client->>BE: POST /api/conversations/(conversationId)/messages {content, clientMessageId}
    BE->>BE: Middleware extract userId
    BE->>BE: Validate conversationId, clientMessageId (UUID); content required, max 4000 chars
    alt Validation fails
        BE-->>Client: 400 Bad Request
    end
    BE->>Postgres: SELECT conversation by id
    alt Not found
        BE-->>Client: 404 Conversation not found
    end
    alt Caller is not user_low/user_high of the conversation
        BE-->>Client: 403 Not a participant of this conversation
    end
    BE->>Postgres: Check caller is still a member of the conversation's server
    alt No longer a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: BEGIN
    BE->>Postgres: INSERT INTO dm_messages ... ON CONFLICT (conversation_id, sender_id, client_message_id) DO NOTHING
    alt clientMessageId already used (duplicate/retry)
        BE->>Postgres: SELECT the existing message row
        BE->>Postgres: COMMIT
        BE-->>Client: 200 DmMessageResponse (the original message, unchanged)
    else new message
        BE->>Postgres: UPDATE dm_conversations SET last_message_at
        BE->>Postgres: UPDATE dm_conversation_states (bump preview + increment unread for the peer only)
        BE->>Postgres: COMMIT
        BE->>WS Hub: Publish "message.new" to the peer (best-effort, no error surfaced to client)
        BE->>FCM: Push notification to the peer's devices (async, best-effort)
        BE-->>Client: 200 DmMessageResponse
    end
```

---

## Notes Redis

Does not use Redis. Real-time delivery goes through the in-process WebSocket hub (`shared.WsHub`/`WsBroker`), not a pub/sub broker — see `websocket_realtime.md`.

---

## Notes Postgres/DB

| Table                     | Column                                                          | Action | Notes                                                                       |
| --------------------------- | ---------------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------- |
| `dm_conversations`        | (lookup)                                                                  | SELECT | Fetch conversation to check participant + server membership                       |
| `server_members`          | (count)                                                                    | SELECT | Re-check the caller is still a member of the conversation's server                 |
| `dm_messages`             | id, conversation_id, sender_id, type, content, client_message_id, created_at | INSERT | Idempotent via `ON CONFLICT (conversation_id, sender_id, client_message_id) DO NOTHING` |
| `dm_messages`             | (lookup by client_message_id)                                              | SELECT | Only run when the insert above found a pre-existing duplicate                     |
| `dm_conversations`        | last_message_at                                                            | UPDATE | Only on a genuinely new message                                                     |
| `dm_conversation_states`  | last_message_at, last_message_preview, unread_count                       | UPDATE | Only on a genuinely new message; `unread_count` is incremented only for the row belonging to the recipient, not the sender |

---

## Prerequisites

Caller must be one of the two participants of `conversationId`, and must still be a member of the server the conversation belongs to.

---

## Request Validation

Path parameter:

| Field             | Type   | Required | Rules           |
| ------------------- | ------ | -------- | --------------- |
| `conversationId`   | string | yes      | Required, UUID  |

JSON body:

| Field              | Type   | Required | Rules                          |
| -------------------- | ------ | -------- | --------------------------------- |
| `content`            | string | yes      | Required, max 4000 characters      |
| `clientMessageId`    | string | yes      | UUID, used as the idempotency key  |

---

## Response

### 200 OK

```json
{
  "id": "message-uuid",
  "conversationId": "conversation-uuid",
  "senderId": "sender-uuid",
  "sender": {
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": null
  },
  "type": "text",
  "content": "hello!",
  "clientMessageId": "client-generated-uuid",
  "createdAt": "2026-06-01T10:00:00Z"
}
```

`type` is always `"text"` currently.

### 400 Bad Request

| `error_message`                              | Cause                       |
| ------------------------------------------------ | -------------------------------- |
| `conversationId is not a valid UUID`             | Invalid `conversationId`          |
| `clientMessageId is not a valid UUID`            | Invalid `clientMessageId`          |
| `content is required`                            | Empty content                     |
| `content must be at most 4000 characters`        | Content too long                   |

### 403 Forbidden

| `error_message`                            | Cause                                                     |
| ---------------------------------------------- | ---------------------------------------------------------------- |
| `Not a participant of this conversation`      | Caller is neither participant of the conversation                  |
| `You are not a member of this server`         | Caller has since left the server the conversation belongs to        |

### 404 Not Found

| `error_message`             | Cause                        |
| -------------------------------- | ---------------------------------- |
| `Conversation not found`        | `conversationId` does not exist     |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 20 July 2026.

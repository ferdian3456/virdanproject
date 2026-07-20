## Overview

This is the real-time WebSocket endpoint used to receive DM events (new messages, read receipts, typing indicators, online/offline presence) without polling. A client opens one connection per session; the server fans events out to it as they happen elsewhere (triggered by `send_dm_message.md` and `mark_conversation_read.md`). Unlike the REST endpoints, authentication is via an access token **query parameter** (browsers cannot set custom headers during the WebSocket handshake).

---

## Auth

Protected, but authenticated differently from the REST API: the access token is passed as `?token=<accessToken>` on the connection URL instead of an `Authorization` header, then validated the same way (JWT signature + a live/non-revoked check).

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant WS Hub

    Client->>BE: GET /api/ws/?token=(accessToken)  (WebSocket upgrade request)
    BE->>BE: Validate token query param present
    alt Missing token
        BE-->>Client: 401 Missing token query parameter
    end
    BE->>BE: Validate JWT + check it's still a live access token
    alt Invalid/expired token
        BE-->>Client: 401 (standard auth error)
    end
    BE->>BE: Require this to actually be a WebSocket upgrade request
    alt Not a WebSocket upgrade (e.g. plain GET)
        BE-->>Client: 426 Upgrade Required
    end
    BE->>WS Hub: Register connection for this userId
    alt Caller already has 5 live connections
        BE-->>Client: WS close, {"type":"error","payload":{"code":"WS_CONN_LIMIT","message":"too many connections"}}
    end
    BE->>WS Hub: Broadcast presence "online" to the caller's DM peers (async)
    loop connection lifetime
        WS Hub-->>Client: {"type":"message.new"|"message.read"|"presence"|"typing", "payload": {...}} (server → client, pushed as events occur elsewhere)
        Client->>BE: {"type":"typing","payload":{"conversationId","isTyping"}} (client → server, optional)
        BE->>WS Hub: Relay typing indicator to the peer (rate-limited to once/second per conversation)
        BE->>Client: Ping frame every 54s (client should respond with a Pong; connection recycled after 60s of silence)
    end
    Client->>BE: Connection closed
    BE->>WS Hub: Unregister connection
    BE->>WS Hub: Broadcast presence "offline" to peers (only if this was the caller's last live connection)
```

---

## Notes Redis

Does not use Redis. Delivery is entirely in-process: a `WsHub` keeps a map of `userId -> connections` held in that server instance's memory, and other requests reach it through the `WsBroker` interface (`InProcessWsBroker`). **In a multi-instance deployment, events are only delivered to a connection held by the same instance that produced the event** — there is currently no cross-instance fanout (e.g. via Redis pub/sub).

---

## Notes Postgres/DB

None directly. Presence broadcast on connect/disconnect reads `dm_conversations` (`GetConversationPeerIds`) to know which peers to notify.

---

## Prerequisites

Caller must present a valid, non-expired access token as the `token` query parameter, and the connecting HTTP request must be a genuine WebSocket upgrade.

---

## Request Validation

Query parameter:

| Field    | Type   | Required | Rules                              |
| -------- | ------ | -------- | -------------------------------------- |
| `token`  | string | yes      | Valid access token (same validity rules as the `Authorization` header on REST endpoints) |

Connection limits:

| Limit                        | Value  | Behavior on exceeding                                     |
| -------------------------------- | ------ | ---------------------------------------------------------------- |
| Max concurrent connections/user  | 5      | The new connection is sent a `WS_CONN_LIMIT` error frame and closed immediately |
| Idle timeout                    | 60s    | Connection is dropped if no Pong is received within 60s of the last ping |
| Ping interval                   | 54s    | Server sends a Ping frame this often to keep the connection alive |
| Max inbound frame size           | 8 KB   | Larger frames close the connection                                |

---

## Server → Client Events

| `type`         | Payload                                                    | Sent when                                                             |
| ---------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| `message.new`   | `DmMessageResponse` (see `send_dm_message.md`)              | The peer sends a new message in a shared conversation                       |
| `message.read`  | `{conversationId, userId, lastReadAt}`                       | The peer marks a shared conversation as read (see `mark_conversation_read.md`) |
| `typing`        | `{conversationId, userId, isTyping}`                         | The peer sends a client → server `typing` frame (relayed, rate-limited to 1/sec per conversation) |
| `presence`      | `{userId, online}`                                           | A DM peer connects (`online: true`) or their last connection drops (`online: false`) |
| `error`         | `{code: "WS_CONN_LIMIT", message: "too many connections"}`  | Immediately before the connection is closed, if the per-user connection limit was exceeded |

## Client → Server Frames

Only one inbound frame type is currently handled; anything else is silently ignored.

| `type`     | Payload                                     | Effect                                                       |
| ------------ | ----------------------------------------------- | ------------------------------------------------------------------ |
| `typing`   | `{conversationId, isTyping}`                    | Relayed to the conversation's other participant as a `typing` event, at most once per second per `(userId, conversationId)` pair |

---

## Response

### On successful upgrade

Standard WebSocket 101 Switching Protocols handshake; no JSON body.

### 401 Unauthorized

| `error_message`                    | Cause                                  |
| -------------------------------------- | -------------------------------------------- |
| `Missing token query parameter`       | No `token` query parameter                     |
| (standard auth error)                 | Token invalid, expired, or revoked              |

### 426 Upgrade Required

Returned if the request reaches this route without actually being a WebSocket upgrade (e.g. a plain browser GET).

---

## Update

This documentation was last updated on 20 July 2026.

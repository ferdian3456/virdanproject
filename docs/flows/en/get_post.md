## Overview

This API is used to fetch a single post by id. The user must be a member of the server where the post resides.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/posts/(postId)
    BE->>BE: Middleware extract userId
    BE->>BE: Validate postId (UUID)
    alt Invalid UUID
        BE-->>Client: 400 postId is not a valid UUID
    end
    BE->>Postgres: SELECT server_id FROM server_posts WHERE id = $1
    alt Post not found
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: COUNT server_members WHERE server_id = $1 AND user_id = $2
    alt Not a member of the server
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: SELECT post detail (author, image, likeCount, commentCount, userLiked, isOwner)
    BE-->>Client: 200 ServerPostResponse
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                    | Column             | Action | Notes                                     |
| ------------------------ | ------------------ | ------ | ----------------------------------------- |
| `server_posts`           | server_id          | SELECT | Fetch server_id from the post              |
| `server_members`         | (count)            | SELECT | Check whether the user is a member of that server          |
| `server_posts`           | (all)              | SELECT | Post detail                                 |
| `server_post_images`     | object_key         | SELECT | Build imageUrl                              |
| `server_post_likes`      | (count + EXISTS)   | SELECT | likeCount + userLiked                       |
| `server_post_comments`   | (count)            | SELECT | commentCount                                |
| `server_member_profiles` | nickname, username, avatar_image_id | SELECT | Author identity in the server   |

---

## Prerequisites

User is a member of the server where the post resides.

---

## Request Validation

Path parameter:

| Field    | Type   | Required | Rules           |
| -------- | ------ | -------- | --------------- |
| `postId` | string | yes      | Required, UUID  |

No body.

---

## Response

### 200 OK

```json
{
  "id": "post-uuid",
  "serverId": "server-uuid",
  "caption": "Hello!",
  "imageUrl": "http://.../webp",
  "mediaType": "image",
  "author": {
    "userId": "user-uuid",
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": "http://.../webp",
    "status": "active"
  },
  "likeCount": 12,
  "commentCount": 3,
  "userLiked": true,
  "userSaved": false,
  "isOwner": false,
  "createdAt": "2026-05-23T10:00:00Z",
  "updatedAt": "2026-05-23T10:00:00Z"
}
```

Note: `videoUrl`, `thumbnailUrl`, `mediaWidth`, `mediaHeight`, and `mirrored` are also present when the post's media is a video.

### 400 Bad Request

| `error_message`               | Cause           |
| ----------------------------- | --------------- |
| `postId is not a valid UUID`  | Invalid UUID    |

### 403 Forbidden

| `error_message`                          | Cause                                   |
| ---------------------------------------- | --------------------------------------- |
| `You are not a member of this server`    | Not a member of the server where the post resides   |

### 404 Not Found

| `error_message`     | Cause               |
| ------------------- | ------------------- |
| `Post not found`    | Post not found       |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 20 July 2026.

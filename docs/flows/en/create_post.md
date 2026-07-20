## Overview

This API is used to create a post in a specific server. The request format is multipart; you must provide **either** an `image` **or** a `video` file (not both, not neither). Images are converted to WebP; videos are probed for duration/dimensions and get an auto-generated WebP thumbnail. Only server members may post. The server's active Virdan Plus subscription raises the allowed upload size limits (it does not block non-Plus members from posting).

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant MinIO

    Client->>BE: POST /api/servers/(serverId)/posts (multipart)
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: COUNT server_members WHERE server_id = $1 AND user_id = $2
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: SELECT active Virdan Plus subscription for serverId (server_plus_orders)
    BE->>BE: Set max image/video size (10MB/50MB free, 100MB/100MB if Plus active)
    BE->>BE: Read "image" and "video" multipart fields
    alt Neither image nor video provided
        BE-->>Client: 400 image or video is required
    end
    alt Both image and video provided
        BE-->>Client: 400 provide either image or video, not both
    end
    BE->>BE: Validate caption (req, max 2000)
    alt Caption invalid
        BE-->>Client: 400 caption is required / caption must be at most 2000 characters
    end
    alt Image branch
        BE->>BE: Validate image (ext jpg/jpeg/png/gif/webp, MIME sniff, size limit), convert to WebP (fit within 1080x1440, no crop)
        BE->>Postgres: BEGIN
        BE->>Postgres: INSERT INTO server_post_images
        BE->>Postgres: INSERT INTO server_posts (post_image_id)
        BE->>MinIO: PutObject server/post/(postImageId).webp
        BE->>Postgres: COMMIT
    else Video branch
        BE->>BE: Validate video (ext mp4/mov/webm, MIME sniff, size limit)
        BE->>BE: Save upload to temp file, ffprobe duration/width/height
        alt Duration exceeds 180s
            BE-->>Client: 400 video duration exceeded 180s limit
        end
        BE->>BE: Generate WebP thumbnail (ffmpeg, quality 75)
        BE->>Postgres: BEGIN
        BE->>Postgres: INSERT INTO server_post_videos (duration, width, height, mirrored)
        BE->>Postgres: INSERT INTO server_posts (post_video_id)
        BE->>MinIO: PutObject server/post/(postVideoId).(ext)
        BE->>MinIO: PutObject server/post/(postVideoId)_thumb.webp
        BE->>Postgres: COMMIT
    end
    BE->>Postgres: SELECT post detail (join author identity, likeCount, commentCount, userLiked, isOwner)
    BE-->>Client: 200 ServerPostResponse
```

---

## Notes Redis

Does not use Redis (other than the auth-check middleware).

---

## Notes MinIO

- Bucket: `MINIO_BUCKET_NAME`
- Image object key: `server/post/{postImageId}.webp`, Content-Type `image/webp`
- Video object key: `server/post/{postVideoId}{ext}` (`.mp4`, `.mov`, or `.webm`), Content-Type `video/mp4` / `video/quicktime` / `video/webm`
- Video thumbnail object key: `server/post/{postVideoId}_thumb.webp`, Content-Type `image/webp`
- Action: PutObject

---

## Notes Postgres/DB

| Table                 | Column                                              | Action | Notes                                                       |
| --------------------- | ---------------------------------------------------- | ------ | ------------------------------------------------------------ |
| `server_members`      | (count)                                              | SELECT | Check whether user is a member                                |
| `server_plus_orders`  | plus_expires_at, status                              | SELECT | Check whether the server has an active Virdan Plus subscription (`status = 'PAID' AND plus_expires_at > now`) |
| `server_post_images`  | id, bucket, object_key, mime_type, size, width, height | INSERT | Image row (image branch only)                                 |
| `server_post_videos`  | id, bucket, object_key, mime_type, size, duration, width, height, thumbnail_object_key, mirrored | INSERT | Video row (video branch only)                                 |
| `server_posts`        | id, server_id, author_id, post_image_id/post_video_id, caption | INSERT | Post row                                                       |

---

## Prerequisites

User is a member of the target server. Has exactly one valid media file (image or video, not both).

---

## Request Validation

Path parameter:

| Field      | Type   | Required | Rules           |
| ---------- | ------ | -------- | --------------- |
| `serverId` | string | yes      | Required, UUID  |

Multipart body:

| Field     | Type    | Required | Rules                                                                                  |
| --------- | ------- | -------- | --------------------------------------------------------------------------------------- |
| `image`   | file    | one of image/video | Image (jpg/jpeg/png/gif/webp), max 10MB (free) / 100MB (server has active Virdan Plus) |
| `video`   | file    | one of image/video | Video (mp4/mov/webm), max 50MB (free) / 100MB (server has active Virdan Plus), max duration 180s |
| `mirror`  | string  | no       | `"true"` to flag the video as mirrored; ignored for image posts                          |
| `caption` | string  | yes      | Required, max 2000 characters                                                            |

---

## Response

### 200 OK

Image post:

```json
{
  "id": "post-uuid",
  "serverId": "server-uuid",
  "caption": "Hello world!",
  "imageUrl": "http://.../server/post/imageId.webp",
  "mediaType": "image",
  "mediaWidth": 1080,
  "mediaHeight": 1350,
  "author": {
    "userId": "user-uuid",
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": "http://.../profile/avatar/uuid.webp",
    "status": "active"
  },
  "likeCount": 0,
  "commentCount": 0,
  "userLiked": false,
  "userSaved": false,
  "isOwner": true,
  "createdAt": "2026-05-23T10:00:00Z",
  "updatedAt": "2026-05-23T10:00:00Z"
}
```

Video post additionally includes `videoUrl`, `thumbnailUrl`, `mediaType: "video"`, and `mirrored` instead of `imageUrl`.

### 400 Bad Request

| `error_message`                              | Cause                          |
| -------------------------------------------- | ------------------------------ |
| `serverId is not a valid UUID`               | UUID invalid                    |
| `image or video is required`                 | Neither file present            |
| `provide either image or video, not both`    | Both files present               |
| `image size exceeded 10MB limit` (or 100MB with active Plus) | Image file too large      |
| `invalid file extension: ...`                | Image extension not allowed      |
| `invalid image type: ...`                    | Image MIME type not allowed      |
| `video size exceeded 50MB limit` (or 100MB with active Plus) | Video file too large      |
| `invalid video extension: ...`               | Video extension not allowed      |
| `invalid video type: ...`                    | Video MIME type not allowed      |
| `video duration exceeded 180s limit`         | Video longer than 180 seconds     |
| `caption is required`                        | Caption empty                   |
| `caption must be at most 2000 characters`    | Caption too long                |

### 403 Forbidden

| `error_message`                          | Cause                   |
| ---------------------------------------- | ----------------------- |
| `You are not a member of this server`    | User is not a server member |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 20 July 2026.

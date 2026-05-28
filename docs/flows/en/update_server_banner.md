## Overview

This API is used to update a server banner. Multipart format, validate image, convert to WebP, upload to MinIO, replace the old banner. Only the owner may do this.

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

    Client->>BE: PUT /api/servers/(id)/banner (multipart, field "banner")
    BE->>BE: Check Content-Type multipart/form-data
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: Check ownership
    alt Not the owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>BE: Fetch FormFile "banner"
    alt File not found
        BE-->>Client: 400 Banner image is required
    end
    BE->>BE: ValidateImage (max 5MB, jpg/png/gif/webp), convert WebP 512x512
    BE->>Postgres: BEGIN
    BE->>Postgres: SELECT old banner_image_id
    BE->>Postgres: INSERT INTO server_banner_images
    BE->>Postgres: UPDATE servers SET banner_image_id = newId
    alt Old banner exists
        BE->>Postgres: DELETE FROM server_banner_images WHERE id = old
    end
    BE->>MinIO: PutObject server/banner/(newId).webp
    BE->>Postgres: COMMIT
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

Does not use Redis.

---

## Notes MinIO

- Bucket: `MINIO_BUCKET_NAME` (default `virdan`)
- Object key: `server/banner/{newImageId}.webp`
- Content-Type: `image/webp`
- Action: PutObject

---

## Notes Postgres/DB

| Table                  | Column                                             | Action | Notes                       |
| ---------------------- | -------------------------------------------------- | ------ | --------------------------- |
| `servers`              | owner_id                                           | SELECT | Check ownership             |
| `servers`              | banner_image_id                                    | SELECT | Fetch old banner             |
| `server_banner_images` | id, bucket, object_key, mime_type, size, ...       | INSERT | New image row                |
| `servers`              | banner_image_id, updated_at, updated_by            | UPDATE | Set new banner               |
| `server_banner_images` | id                                                 | DELETE | Delete old image             |

---

## Prerequisites

The user is the server owner. Has a valid image file.

---

## Request Validation

Format: `multipart/form-data`.

| Field        | Type   | Required | Rules                                   |
| ------------ | ------ | -------- | --------------------------------------- |
| `id` (path)  | string | yes      | Required, UUID                           |
| `banner`     | file   | yes      | Image (jpg/jpeg/png/gif/webp), max 5MB  |

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

### 400 Bad Request

| `error_message`                                                       | Cause                          |
| --------------------------------------------------------------------- | ------------------------------ |
| `Invalid Content-Type header. Endpoint requires multipart/form-data.` | Content-Type is not multipart  |
| `serverId is not a valid UUID`                                        | UUID invalid                    |
| `Banner image is required`                                            | Banner file not found           |
| `image size exceeded 5MB limit`                                       | File too large                  |
| `invalid file extension: ...`                                         | Extension not allowed           |
| `invalid image type: ...`                                             | MIME type not allowed           |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not the owner of this server`   | Not the owner |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.

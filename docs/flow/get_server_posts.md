## Get Server Posts Flow

### Overview
Retrieves a paginated list of posts from a specific server. Requires server membership. Uses cursor-based pagination.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path — must be a valid UUID
3. Parse query parameters: `limit`, `cursor`
4. Validate `limit` (>= 0 and <= max limit)
5. Check if user is a member of the server
6. If cursor provided, decode and unmarshal
7. Query posts from database (limit + 1 for cursor detection) with author info, image URLs, and like status for current user
8. Build next cursor if more data exists

### Database Operations

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = $3
```

#### PostgreSQL — Get Server Posts (first page)
```sql
SELECT sp.author_id, us.username, uai.object_key, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
       COALESCE(comment_counts.comment_count, 0) as comment_count,
       COALESCE(like_counts.like_count, 0) as like_count,
       user_likes.user_id is not null as is_liked
FROM server_posts sp
INNER JOIN users us ON sp.author_id = us.id
LEFT JOIN user_avatar_images uai ON us.id = uai.user_id
INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
LEFT JOIN (
    SELECT post_id, COUNT(*) as comment_count FROM server_post_comments GROUP BY post_id
) comment_counts ON sp.id = comment_counts.post_id
LEFT JOIN (
    SELECT post_id, COUNT(*) as like_count FROM server_post_likes GROUP BY post_id
) like_counts ON sp.id = like_counts.post_id
LEFT JOIN server_post_likes user_likes ON sp.id = user_likes.post_id AND user_likes.user_id = $3
WHERE sp.server_id = $1
ORDER BY sp.create_datetime DESC, sp.id DESC
LIMIT $2
```

#### PostgreSQL — Get Server Posts (with cursor)
Same query with additional cursor filter:
```sql
WHERE sp.server_id = $1
AND (sp.create_datetime < $2 OR (sp.create_datetime = $2 AND sp.id < $3))
ORDER BY sp.create_datetime DESC, sp.id DESC
LIMIT $4
```
- **Tables**: `server_posts`, `users`, `user_avatar_images`, `server_post_images`, `server_post_comments` (aggregated), `server_post_likes` (aggregated + current user check)
- **Cursor**: keyset pagination on `(create_datetime, id)` DESC
- **Columns returned**: `author_id`, `username`, author avatar `object_key`, post `id`, post image `object_key`, `caption`, timestamps, `comment_count`, `like_count`, `is_liked`
- **`is_liked`**: `true` if current user has liked the post, `false` otherwise

#### MinIO — Image URL Construction
```
Post image: {MINIO_FULL_URL}/{object_key}  (e.g., server/post/{imageId}.webp)
Author avatar: {MINIO_FULL_URL}/{object_key}  (e.g., user/avatar/{imageId}.webp)
```

### Query Parameters
- `limit` — number of items per page (optional)
- `cursor` — base64-encoded cursor for next page (optional)

### Response Format
```json
{
  "data": [
    {
      "postId": "uuid",
      "ownerId": "uuid",
      "ownerName": "username",
      "ownerImageUrl": "http://...",
      "postImageUrl": "http://...",
      "caption": "Post caption",
      "commentCount": 5,
      "likeCount": 42,
      "isLiked": true,
      "createDatetime": "2024-01-15T10:30:00Z",
      "updateDatetime": "2024-01-15T10:30:00Z"
    }
  ],
  "page": {
    "nextCursor": "base64_cursor_or_null"
  }
}
```

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Limit < 0 → `400` with "Limit must be greater or equal than 0"
- Limit > max → `400` with "Limit is exceeded max limit: {max}"
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse Params → Validate Limit → Check Membership (DB) → Decode Cursor → Query Posts with is_liked (DB) → Build Next Cursor → Response
```

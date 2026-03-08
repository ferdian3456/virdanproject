## Create Post Flow

### Overview
Creates a new post in a server. Requires server membership. Posts must include an image (uploaded as multipart form) and a caption.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path — must be a valid UUID
3. Check if user is a member of the server
4. Read `image` file from multipart form (required, cannot be empty)
5. Validate image file (format, size)
6. Read `caption` from form value (required)
7. Begin database transaction
8. Upload image to MinIO (`server/post/{imageId}.webp`)
9. Create post image record in database
10. Create post record in database
11. Commit transaction
12. Fetch and return full post details including author info and image URL

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Not a member → `400` with "You are not a member of this server"
- No image file → `400` with "Image is required"
- Invalid image format → `400` validation error
- Caption empty → `400` with "Caption is required"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Membership (DB) → Read Image → Validate Image → Read Caption → Begin TX → Upload Image (MinIO) → Create Image Record (DB) → Create Post (DB) → Commit TX → Get Post (DB) → Response
```

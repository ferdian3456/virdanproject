## Update Avatar Flow

### Overview
Updates the authenticated user's avatar image. Accepts a multipart form file upload. The image is validated, converted to WebP, and stored in MinIO object storage.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Read `avatar` field from multipart form data
3. Validate the image file (format, size)
4. Begin database transaction
5. Check if user already has an avatar image
6. If old avatar exists:
   - Delete old avatar record from database
   - Delete old avatar file from MinIO
7. Create new avatar image record in database (with new UUID, bucket, object key)
8. Upload new avatar file to MinIO (`user/avatar/{imageId}.webp`)
9. Commit transaction
10. Return success with no data

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Missing avatar file → `400` with "Avatar is required to not be empty"
- Invalid image format/size → `400` validation error
- MinIO upload failure → `500` internal server error

### Flow
```
Request → Auth Middleware → Get Avatar File → Validate Image → Begin TX → Check Old Avatar → Delete Old (DB + MinIO) → Create New Record (DB) → Upload (MinIO) → Commit TX → Response
```

## Update Server Category Flow

### Overview
Updates the category of a server. Only the server owner can perform this action. Set `categoryId` to `null` to remove the category.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. If `categoryId` is provided (not null), verify the category exists in database
4. Check server ownership
5. Update category in database
6. Return updated server details

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Category not found → `400` with "Category id is not found"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Category Exists (DB) → Check Ownership (DB) → Update (DB) → Get Detail (DB) → Response
```

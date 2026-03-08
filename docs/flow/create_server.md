## Create Server Flow

### Overview
Creates a new server (community). The creator automatically becomes the owner with full permissions. A server role and server member record are also created in a transaction.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse and validate request body
3. Validate `name` (required, 5–40 characters)
4. Validate `shortName` (required, 5–10 characters)
5. If `categoryId` is provided, verify category exists in database
6. Create server record with JSON settings (`isPrivate`)
7. Create owner role with full permissions (`{"*": true}`)
8. Create server member record linking user to server with owner role
9. All operations wrapped in a database transaction
10. Return created server details

### Error Cases
- Name empty → `400` with "Name is required to not be empty"
- Name < 5 chars → `400` with "Name must be at least 4 characters"
- Name > 40 chars → `400` with "Name must be at most 40 characters"
- ShortName empty → `400` with "Short name is required to not be empty"
- ShortName < 5 chars → `400` with "Short name must be at least 5 characters"
- ShortName > 10 chars → `400` with "Short name must be at most 10 characters"
- Invalid categoryId → `400` with "Category id is not found"

### Flow
```
Request → Auth Middleware → Validate Body → Check Category (DB) → Begin TX → Create Server (DB) → Create Role (DB) → Create Member (DB) → Commit TX → Response
```

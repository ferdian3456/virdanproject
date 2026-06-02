# Plan — Save Post (Bookmark) — VIR-XXX

> Status: FINAL (post-review). Siap eksekusi setelah Jira task dibuat.
> Fitur: user bisa save/unsave post (bookmark privat) + lihat saved feed **per-server**.

---

## 1. Keputusan Terkunci

| # | Keputusan | Nilai |
|---|---|---|
| D1 | Pemilik save | `user_id` (global owner, bukan `profile_id`) |
| D2 | Member-guard save & unsave | Ya. Non-member → 403 (sama pola `LikePost`) |
| D3 | Saved feed scope | **Per-server**, `GET /servers/:serverId/posts/saved`, member-guarded. TIDAK cross-server (Virdan server-tied). |
| D4 | Notifikasi | Tidak ada (aksi privat) |
| D5 | Urutan saved feed | By `server_post_saves.created_at DESC` (waktu save terbaru di atas) |
| D6 | Save count publik | Tidak ada |
| D7 | Save post yang sudah di-save | Validasi eksplisit `CheckPostSave` → `ConflictError` 409. Bukan `ON CONFLICT`. |
| D8 | Unsave post yang belum di-save | Validasi `CheckPostSave` → `NotFoundError` 404 |
| D9 | Unique index `(post_id, user_id)` | Tetap dipertahankan sebagai jaring pengaman DB (anti-race) |
| D10 | Post author leave/delete | Post (dan save-nya) tetap ada; ditandai `author_status` (`user_left`/`user_deleted`). Verified dari `post_repository.go:244-248`. |

---

## 2. Ringkasan Perubahan File

| Layer | File | Aksi |
|---|---|---|
| schema | `db/schema.sql` | Tambah tabel `server_post_saves` |
| migration | `db/migrations/<ts>_create_server_post_saves.sql` | Auto-gen via `make migrate-diff` |
| model | `internal/model/server_post_saves.go` | **Baru** — `ServerPostSave`, `PostSaveResponse`, `SavedPostCursor` |
| model | `internal/model/server_posts.go` | Tambah field `UserSaved` + `SavedAt` di `ServerPostResponse` |
| repository | `internal/repository/post_repository.go` | Tambah `CreatePostSave`, `CheckPostSave`, `DeletePostSave`, `GetSavedPosts`; tambah `user_saved` EXISTS di 4 query feed |
| usecase | `internal/usecase/post_usecase.go` | Tambah `SavePost`, `UnsavePost`, `GetSavedPosts` |
| controller | `internal/delivery/http/post_controller.go` | Tambah `SavePost`, `UnsavePost`, `GetSavedPosts` |
| route | `internal/delivery/http/route/route.go` | Tambah 3 route |
| docs | `docs/flows/{id,en}/save_post.md`, `unsave_post.md`, `get_saved_posts.md` | **Baru** |
| test | `tests/integration/post/save_post_test.go` | **Baru** |

**Tidak perlu** wiring baru di `cmd/main.go` — method di `PostUsecase` yang sudah ter-inject.

Endpoint:
```
POST   /api/posts/:postId/saves              → save post
DELETE /api/posts/:postId/saves              → unsave post
GET    /api/servers/:serverId/posts/saved    → saved feed per-server (cursor paginated)
```

---

## 3. Schema (`db/schema.sql`)

Tambah setelah `server_post_likes`:

```sql
CREATE TABLE IF NOT EXISTS server_post_saves (
    id          uuid PRIMARY KEY,
    post_id     uuid NOT NULL,
    user_id     uuid NOT NULL,
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    created_by  uuid NOT NULL,
    updated_by  uuid NOT NULL,
    FOREIGN KEY (post_id) REFERENCES server_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id)        ON DELETE CASCADE
);

-- D9: jaring pengaman idempotensi (1 save per user per post), anti-race
CREATE UNIQUE INDEX IF NOT EXISTS idx_server_post_saves_uk_01 ON server_post_saves(post_id, user_id);
-- saved feed per-server: filter user_id + server-nya, urut waktu save desc.
-- server_id tidak ada di tabel ini → join ke server_posts. Index composite di kolom
-- yang tersedia: (user_id, created_at). server_id difilter via join (lihat catatan).
CREATE INDEX IF NOT EXISTS idx_server_post_saves_pk_01 ON server_post_saves(user_id, created_at DESC);
```

> **Catatan index (Q3)**: tabel `server_post_saves` tidak menyimpan `server_id` (post_id sudah cukup, server_id ada di `server_posts`). Jadi composite `(user_id, server_id, created_at)` TIDAK bisa langsung di tabel ini. Filter `server_id` terjadi via JOIN ke `server_posts`. Index `(user_id, created_at DESC)` menutup pencarian semua save milik user + ordering; filter server_id dilakukan setelah join (jumlah save per user kecil, efisien). Kalau nanti volume save per user besar dan butuh optimasi server-scoped, opsi: denormalisasi `server_id` ke `server_post_saves` lalu index `(user_id, server_id, created_at DESC)` — catat sebagai kemungkinan tech debt, belum perlu sekarang.

Generate migration:
```bash
make migrate-diff name=create_server_post_saves
# review db/migrations/<ts>_create_server_post_saves.sql
make migrate-apply
```

---

## 4. Model

### 4.1 File baru: `internal/model/server_post_saves.go`

```go
package model

import (
	"time"
)

type ServerPostSave struct {
	Id        string
	PostId    string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type PostSaveResponse struct {
	PostId    string `json:"postId"`
	UserSaved bool   `json:"userSaved"`
}

// SavedPostCursor paginates the saved feed by save time (server_post_saves.created_at),
// newest-save-first — NOT by post creation time.
type SavedPostCursor struct {
	SavedAt time.Time `json:"savedAt"`
	PostId  string    `json:"postId"`
}
```

### 4.2 Edit: `internal/model/server_posts.go`

Tambah 2 field di `ServerPostResponse` (setelah `UserLiked`):

```go
type ServerPostResponse struct {
	Id           string                 `json:"id"`
	ServerId     string                 `json:"serverId"`
	Caption      string                 `json:"caption"`
	ImageUrl     *string                `json:"imageUrl"`
	Author       AuthorIdentityResponse `json:"author"`
	LikeCount    int                    `json:"likeCount"`
	CommentCount int                    `json:"commentCount"`
	UserLiked    bool                   `json:"userLiked"`
	UserSaved    bool                   `json:"userSaved"`           // NEW: bookmark state untuk ikon terisi
	SavedAt      *time.Time             `json:"savedAt,omitempty"`   // NEW: label JSON = sps.created_at; hanya terisi di saved feed (untuk cursor)
	IsOwner      bool                   `json:"isOwner"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}
```

> `SavedAt` = nilai `server_post_saves.created_at` (waktu save), BUKAN `created_at` post. Beda timestamp. `omitempty` → tak muncul di feed biasa, cuma dipakai di saved feed untuk bentuk next cursor. Tidak ada kolom `saved_at` di tabel — ini murni label response.

---

## 5. Repository (`internal/repository/post_repository.go`)

### 5.1 Tambah `user_saved` EXISTS di 4 query feed

Tambah 1 baris EXISTS setelah `user_liked`, dan 1 target scan `&resp.UserSaved` PERSIS setelah `&resp.UserLiked`. **Param index tidak bergeser** (reuse param userId yang ada).

- **`GetPost`** (`$2`=userId): EXISTS pakai `$2`.
- **`GetServerPosts`** (`$2`=userId): EXISTS pakai `$2`.
- **`GetServerPostForMe`** (`$2`=userId): EXISTS pakai `$2`.
- **`GetServerPostsByAuthor`** (`$3`=requesterId): EXISTS pakai `$3` (viewer, bukan author).

Contoh di `GetPost` SELECT:
```sql
				EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $2) AS user_liked,
				EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $2) AS user_saved
```
Scan: `&resp.UserLiked,` lalu `&resp.UserSaved,`.

### 5.2 `CreatePostSave` (plain INSERT, return error saja)

```go
func (repository *PostRepository) CreatePostSave(ctx context.Context, save model.ServerPostSave) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreatePostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("post.id", save.PostId),
		attribute.String("user.id", save.UserId),
	)

	query := `INSERT INTO server_post_saves (id, post_id, user_id, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = repository.DB.Exec(ctx, query, save.Id, save.PostId, save.UserId,
		save.CreatedAt, save.UpdatedAt, save.CreatedBy, save.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create post save", zap.Error(err))
		return err
	}

	return nil
}
```

> Tanpa `ON CONFLICT` (D7: cek duplikat di usecase). Unique index `idx_server_post_saves_uk_01` tetap ada sebagai jaring pengaman (D9).

### 5.3 `CheckPostSave`

```go
func (repository *PostRepository) CheckPostSave(ctx context.Context, postId string, userId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckPostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `SELECT EXISTS (SELECT 1 FROM server_post_saves WHERE post_id = $1 AND user_id = $2)`

	var exists bool
	err = repository.DB.QueryRow(ctx, query, postId, userId).Scan(&exists)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check post save", zap.Error(err))
		return false, err
	}

	return exists, nil
}
```

### 5.4 `DeletePostSave`

```go
func (repository *PostRepository) DeletePostSave(ctx context.Context, postId string, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeletePostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM server_post_saves WHERE post_id = $1 AND user_id = $2`

	_, err = repository.DB.Exec(ctx, query, postId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete post save", zap.Error(err))
		return err
	}

	return nil
}
```

### 5.5 `GetSavedPosts` (per-server)

```go
// GetSavedPosts returns the viewer's saved posts WITHIN one server, newest-save-first.
// Member-guard dilakukan di usecase (CheckServerMember) sebelum query ini dipanggil.
// UserSaved selalu true (ini saved feed); SavedAt = sps.created_at untuk cursor.
func (repository *PostRepository) GetSavedPosts(ctx context.Context, limit int, serverId, userId string, cursor *model.SavedPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetSavedPosts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $1) AS user_liked,
			sps.created_at AS saved_at
		FROM server_post_saves sps
		INNER JOIN server_posts sp ON sp.id = sps.post_id
		INNER JOIN users u ON sp.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.PostId != "" && !cursor.SavedAt.IsZero() {
		query := baseSelect + `
		WHERE sps.user_id = $1 AND sp.server_id = $2
		AND (sps.created_at < $3 OR (sps.created_at = $3 AND sps.post_id < $4))
		ORDER BY sps.created_at DESC, sps.post_id DESC
		LIMIT $5`
		rows, err = repository.DB.Query(ctx, query, userId, serverId, cursor.SavedAt, cursor.PostId, limit)
	} else {
		query := baseSelect + `
		WHERE sps.user_id = $1 AND sp.server_id = $2
		ORDER BY sps.created_at DESC, sps.post_id DESC
		LIMIT $3`
		rows, err = repository.DB.Query(ctx, query, userId, serverId, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query saved posts", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		var authorStatus string
		var savedAt time.Time
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&authorStatus,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&savedAt,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan saved post row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatus(authorStatus)
		resp.IsOwner = resp.Author.UserId == userId
		resp.UserSaved = true
		savedAtCopy := savedAt
		resp.SavedAt = &savedAtCopy

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}
```

---

## 6. Usecase (`internal/usecase/post_usecase.go`)

### 6.1 `SavePost`

```go
func (usecase *PostUsecase) SavePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostSaveResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.SavePost")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := usecase.PostRepository.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return model.PostSaveResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.PostSaveResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.PostSaveResponse{}, err
	}

	// D7: tolak save ganda secara eksplisit (unique index = jaring pengaman terakhir)
	saveExists, err := usecase.PostRepository.CheckPostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return model.PostSaveResponse{}, err
	}
	if saveExists {
		err = &model.ConflictError{Code: constant.ERR_CONFLICT_CODE, Message: "Post sudah disimpan", Param: "postId"}
		return model.PostSaveResponse{}, err
	}

	now := time.Now().UTC()
	postSave := model.ServerPostSave{
		Id:        uuid.New().String(),
		PostId:    postIdParam,
		UserId:    userId,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	err = usecase.PostRepository.CreatePostSave(ctxContext, postSave)
	if err != nil {
		return model.PostSaveResponse{}, err
	}

	return model.PostSaveResponse{PostId: postIdParam, UserSaved: true}, nil
}
```

### 6.2 `UnsavePost`

```go
func (usecase *PostUsecase) UnsavePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostSaveResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UnsavePost")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := usecase.PostRepository.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return model.PostSaveResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.PostSaveResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.PostSaveResponse{}, err
	}

	// D8: hanya bisa unsave kalau memang sudah di-save
	saveExists, err := usecase.PostRepository.CheckPostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return model.PostSaveResponse{}, err
	}
	if !saveExists {
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Post belum disimpan", Param: "postId"}
		return model.PostSaveResponse{}, err
	}

	err = usecase.PostRepository.DeletePostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return model.PostSaveResponse{}, err
	}

	return model.PostSaveResponse{PostId: postIdParam, UserSaved: false}, nil
}
```

### 6.3 `GetSavedPosts` (per-server, member-guarded)

```go
func (usecase *PostUsecase) GetSavedPosts(ctx fiber.Ctx, serverId string, userId string) (model.ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(constant.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetSavedPosts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.ServerPostListResponse{}, err
	}

	var cursor model.SavedPostCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.SavedPostCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	posts, err := usecase.PostRepository.GetSavedPosts(ctxContext, limit+1, serverId, userId, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	response := model.ServerPostListResponse{Data: []model.ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.SavedPostCursor{
			SavedAt: *last.SavedAt,
			PostId:  last.Id,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}
```

---

## 7. Controller (`internal/delivery/http/post_controller.go`)

```go
// SavePost godoc
// @Summary      Save (bookmark) a post
// @Description.markdown save_post
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.PostSaveResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      409   {object}  model.ConflictError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/saves [post]
func (controller *PostController) SavePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.SavePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response model.PostSaveResponse
	response, err = controller.PostUsecase.SavePost(ctx, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UnsavePost godoc
// @Summary      Unsave (remove bookmark) a post
// @Description.markdown unsave_post
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.PostSaveResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      404   {object}  model.NotFoundError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/saves [delete]
func (controller *PostController) UnsavePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UnsavePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response model.PostSaveResponse
	response, err = controller.PostUsecase.UnsavePost(ctx, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetSavedPosts godoc
// @Summary      Get saved posts in a server
// @Description.markdown get_saved_posts
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerPostListResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts/saved [get]
func (controller *PostController) GetSavedPosts(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetSavedPosts")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response model.ServerPostListResponse
	response, err = controller.PostUsecase.GetSavedPosts(ctx, serverId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}
```

---

## 8. Route (`internal/delivery/http/route/route.go`)

### 8.1 Save/Unsave — di `postGroup` (sebelah likes)

```go
	postGroup.Post("/:postId/saves", c.PostController.SavePost)     // NEW
	postGroup.Delete("/:postId/saves", c.PostController.UnsavePost) // NEW
```

### 8.2 Saved feed — di `serverGroup`, taruh sebelah `/:serverId/posts/me`

```go
	serverGroup.Get("/:serverId/posts/me", c.PostController.GetServerPostForMe)
	serverGroup.Get("/:serverId/posts/saved", c.PostController.GetSavedPosts) // NEW
```

> **Ordering Fiber**: `/posts/saved` literal, aman — tak ada route `GET /:serverId/posts/:postId` di `serverGroup` (single post GET ada di `postGroup`). Sejajar pola `/posts/me`.

---

## 9. Swagger Flow Docs

Buat 3 file di `docs/flows/id/` DAN `docs/flows/en/` (nama harus sama persis dengan tag `@Description.markdown`):

`save_post.md`, `unsave_post.md`, `get_saved_posts.md`.

Contoh `docs/flows/id/save_post.md`:
```markdown
Menyimpan (bookmark) sebuah post ke daftar simpanan pribadi user di dalam server.

- Wajib login (Bearer token).
- User wajib member dari server tempat post berada (403 jika bukan).
- Jika post sudah disimpan sebelumnya, mengembalikan 409 (Post sudah disimpan).
- Save bersifat privat dan tidak mengirim notifikasi ke pemilik post.
```

Lalu `make generate-swagger-id` (opsional preview; CI regenerate otomatis).

---

## 10. Integration Test (`tests/integration/post/save_post_test.go`)

| Test | Ekspektasi |
|---|---|
| `TestSavePost_Success` | 200, `userSaved=true` |
| `TestSavePost_AlreadySaved` | 409 (`ERR_CONFLICT_CODE`) |
| `TestSavePost_NotAMember` | 403 (`ERR_FORBIDDEN_CODE`) |
| `TestSavePost_InvalidPostId` | 400 (UUID invalid) |
| `TestSavePost_PostNotFound` | 404 (UUID valid tapi post tak ada) |
| `TestSavePost_Unauthorized` | 401 (tanpa token) |
| `TestUnsavePost_Success` | 200, `userSaved=false` |
| `TestUnsavePost_NotSaved` | 404 (`ERR_NOT_FOUND_CODE`) — belum di-save |
| `TestUnsavePost_NotAMember` | 403 |
| `TestGetSavedPosts_Success` | hanya post yang di-save di server itu, urut save desc |
| `TestGetSavedPosts_Pagination` | cursor benar, tidak ada item ke-skip/duplikat |
| `TestGetSavedPosts_OnlyThisServer` | save di server A tidak muncul di saved feed server B |
| `TestGetSavedPosts_NotAMember` | 403 |
| `TestGetSavedPosts_PostDeletedCascade` | post dihapus → hilang dari saved feed (cascade) |
| `TestGetSavedPosts_AuthorLeft` | post author leave → tetap muncul, `author.status='user_left'` (D10) |
| `TestFeed_UserSavedFlag` | `GET server posts` → `userSaved` true untuk post yang di-save, false lainnya |

> Jangan cuma happy path (aturan QA): sertakan edge + failure di atas.

---

## 11. Update `virdanproject/CLAUDE.md` (setelah merge)

- Tambah `server_post_saves` ke **Tables Overview** + DDL carousel (script `000018` atau sesuai urutan migration).
- Tambah block **Post Interactions** route: `POST/DELETE /posts/:postId/saves`, `GET /servers/:serverId/posts/saved`.
- Catat field baru `userSaved`/`savedAt` di response post.

---

## 12. Urutan Eksekusi

1. Search Jira VIR → buat task `Task` (project VIR tak punya `Bug`) → dapat `VIR-XXX`.
2. Branch `VIR-XXX` dari `origin/main` (local sering stale).
3. Schema + `make migrate-diff` + review migration + `make migrate-apply`.
4. Model → repository → usecase → controller → route.
5. Swagger flow docs.
6. `make test-post` (minta izin dulu — test ~10 menit).
7. Commit `VIR-XXX: <title>` → PR → update CLAUDE.md.

---

## 13. Risiko / Catatan Review

- **Blast radius**: 4 query feed disentuh (`user_saved`). Mitigasi: `&resp.UserSaved` SELALU tepat setelah `&resp.UserLiked`, SQL `user_saved` tepat setelah `user_liked`. Reuse param `$2`/`$3`, tak ada renumber.
- **Race save ganda (TOCTOU)**: validasi app (`CheckPostSave`) tak menutup race 2 request bersamaan. Penutup nyata = unique index `idx_server_post_saves_uk_01` (D9). Kalau race kejadian, insert ke-2 kena unique violation → saat ini muncul 500. Jarang (FE cegah double-tap). Perbaikan opsional nanti: map pgErr `23505` → 409. Di luar scope.
- **Performa saved feed**: 2 subquery COUNT + 1 EXISTS per row, sama pola feed existing (terbukti dipakai). Index `(user_id, created_at DESC)` nutup pencarian save milik user + ordering; filter `server_id` via join. Untuk volume save besar per user, opsi denormalisasi `server_id` ke tabel saves → tech debt, belum perlu.
- **`SavedAt` di struct umum** (`omitempty`): muncul cuma di saved feed. Trade-off kecil vs struct terpisah. Tetap pilih `omitempty` (minim duplikasi scan).
```

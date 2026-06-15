package usecase

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type PostUsecase struct {
	PostRepository       *repository.PostRepository
	ServerRepository     *repository.ServerRepository
	ProfileRepository    *repository.ProfileRepository
	ServerPlusRepository *repository.ServerPlusRepository
	NotificationUsecase  *NotificationUsecase
	DB                   *pgxpool.Pool
	Log                  *zap.Logger
	Config               *koanf.Koanf
}

func NewPostUsecase(
	postRepository *repository.PostRepository,
	serverRepository *repository.ServerRepository,
	profileRepository *repository.ProfileRepository,
	serverPlusRepository *repository.ServerPlusRepository,
	notificationUsecase *NotificationUsecase,
	db *pgxpool.Pool,
	zap *zap.Logger,
	koanf *koanf.Koanf,
) *PostUsecase {
	return &PostUsecase{
		PostRepository:       postRepository,
		ServerRepository:     serverRepository,
		ProfileRepository:    profileRepository,
		ServerPlusRepository: serverPlusRepository,
		NotificationUsecase:  notificationUsecase,
		DB:                   db,
		Log:                  zap,
		Config:               koanf,
	}
}

func (usecase *PostUsecase) CreatePost(ctx fiber.Ctx, serverId string, userId string) (model.ServerPostResponse, error) {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.CreatePost")
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
	)

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.ServerPostResponse{}, err
	}

	// Virdan Plus raises this server's per-post upload size limits (time-limited).
	plusActive, _, plusErr := usecase.ServerPlusRepository.GetActivePlus(ctxContext, serverId)
	if plusErr != nil {
		err = plusErr
		return model.ServerPostResponse{}, err
	}
	maxImageSize := int64(constant.MAX_IMAGE_SIZE_FREE)
	maxVideoSize := int64(constant.MAX_VIDEO_SIZE_FREE)
	if plusActive {
		maxImageSize = int64(constant.MAX_IMAGE_SIZE_PLUS)
		maxVideoSize = int64(constant.MAX_VIDEO_SIZE_PLUS)
	}

	// Detect media type from form fields
	imageHeader, imageErr := ctx.FormFile("image")
	hasImage := imageErr == nil && imageHeader != nil
	videoHeader, videoErr := ctx.FormFile("video")
	hasVideo := videoErr == nil && videoHeader != nil

	if !hasImage && !hasVideo {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "image or video is required", Param: "image"}
		return model.ServerPostResponse{}, err
	}
	if hasImage && hasVideo {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "provide either image or video, not both", Param: "image"}
		return model.ServerPostResponse{}, err
	}

	caption := ctx.FormValue("caption")
	// Front-camera clips record un-mirrored; the FE flags them so playback can
	// be mirrored to match the selfie preview the user framed against. Only the
	// video branch consumes this.
	mirrorVideo := ctx.FormValue("mirror") == "true"
	v.Reset()
	v.String("caption", caption).Required().MaxLen(2000)
	if valErr := v.Validate(); valErr != nil {
		err = valErr
		return model.ServerPostResponse{}, err
	}

	now := time.Now().UTC()
	postId := uuid.New().String()
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	if hasImage {
		// ── IMAGE FLOW ──
		imageFile, imageSize, imageWidth, imageHeight, imgErr := util.ValidateImage(ctxContext, imageHeader, "image", maxImageSize, 1080, 1350, false)
		if imgErr != nil {
			err = imgErr
			return model.ServerPostResponse{}, err
		}

		postImageId := uuid.New().String()
		objectKey := fmt.Sprintf("server/post/%s.webp", postImageId)

		serverPostImage := model.ServerPostImage{
			Id:        postImageId,
			Bucket:    bucketName,
			ObjectKey: objectKey,
			MimeType:  "image/webp",
			Size:      imageSize,
			Width:     imageWidth,
			Height:    imageHeight,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		serverPost := model.ServerPost{
			Id: postId, ServerId: serverId, AuthorId: userId,
			PostImageId: &postImageId, Caption: caption,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		committed := false
		tx, txErr := usecase.DB.Begin(ctxContext)
		if txErr != nil {
			err = txErr
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
			return model.ServerPostResponse{}, err
		}
		defer func() {
			if !committed {
				_ = tx.Rollback(ctxContext)
			}
		}()

		if err = usecase.PostRepository.CreateServerPostImage(ctxContext, tx, serverPostImage); err != nil {
			return model.ServerPostResponse{}, err
		}
		if err = usecase.PostRepository.CreateServerPost(ctxContext, tx, serverPost); err != nil {
			return model.ServerPostResponse{}, err
		}
		if err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, objectKey, imageFile, imageSize, "image/webp"); err != nil {
			return model.ServerPostResponse{}, err
		}
		if err = tx.Commit(ctxContext); err != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
			return model.ServerPostResponse{}, err
		}
		committed = true

	} else {
		// ── VIDEO FLOW ──
		if vErr := util.ValidateVideoFile(ctxContext, videoHeader, "video", maxVideoSize); vErr != nil {
			err = vErr
			return model.ServerPostResponse{}, err
		}

		tmpDir := "/app/tmp"
		tmpPath, tmpFile, saveErr := util.SaveMultipartToTemp(videoHeader, tmpDir, "vid-"+postId)
		if saveErr != nil {
			err = saveErr
			return model.ServerPostResponse{}, err
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}()

		duration, videoWidth, videoHeight, probeErr := util.ProbeVideoMetadata(ctxContext, tmpPath)
		if probeErr != nil {
			err = probeErr
			return model.ServerPostResponse{}, err
		}

		if duration > constant.MAX_VIDEO_DURATION {
			err = &model.BadRequestError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "video duration exceeded " + strconv.Itoa(constant.MAX_VIDEO_DURATION) + "s limit",
				Param:   "video",
			}
			return model.ServerPostResponse{}, err
		}

		thumbnailBytes, thumbErr := util.GenerateVideoThumbnail(ctxContext, tmpPath, 75)
		if thumbErr != nil {
			err = thumbErr
			return model.ServerPostResponse{}, err
		}

		postVideoId := uuid.New().String()
		ext := filepath.Ext(videoHeader.Filename)
		videoObjectKey := fmt.Sprintf("server/post/%s%s", postVideoId, ext)
		thumbObjectKey := fmt.Sprintf("server/post/%s_thumb.webp", postVideoId)

		// Determine MIME from extension
		mimeType := "video/mp4"
		switch strings.ToLower(ext) {
		case ".mov":
			mimeType = "video/quicktime"
		case ".webm":
			mimeType = "video/webm"
		}

		serverPostVideo := model.ServerPostVideo{
			Id: postVideoId, Bucket: bucketName, ObjectKey: videoObjectKey,
			MimeType: mimeType, Size: videoHeader.Size,
			Duration: duration, Width: videoWidth, Height: videoHeight,
			ThumbnailObjectKey: thumbObjectKey,
			Mirrored:           mirrorVideo,
			CreatedAt:          now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		serverPost := model.ServerPost{
			Id: postId, ServerId: serverId, AuthorId: userId,
			PostVideoId: &postVideoId, Caption: caption,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		committed := false
		tx, txErr := usecase.DB.Begin(ctxContext)
		if txErr != nil {
			err = txErr
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
			return model.ServerPostResponse{}, err
		}
		defer func() {
			if !committed {
				_ = tx.Rollback(ctxContext)
			}
		}()

		if err = usecase.PostRepository.CreateServerPostVideo(ctxContext, tx, serverPostVideo); err != nil {
			return model.ServerPostResponse{}, err
		}
		if err = usecase.PostRepository.CreateServerPost(ctxContext, tx, serverPost); err != nil {
			return model.ServerPostResponse{}, err
		}

		// Upload video file from disk
		videoFile, openErr := os.Open(tmpPath)
		if openErr != nil {
			err = openErr
			return model.ServerPostResponse{}, err
		}
		defer func() { _ = videoFile.Close() }()

		videoBytes, readErr := io.ReadAll(videoFile)
		if readErr != nil {
			err = readErr
			return model.ServerPostResponse{}, err
		}
		videoReader := bytes.NewReader(videoBytes)

		if err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, videoObjectKey, videoReader, int64(len(videoBytes)), mimeType); err != nil {
			return model.ServerPostResponse{}, err
		}

		// Upload thumbnail
		thumbReader := bytes.NewReader(thumbnailBytes)
		if err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, thumbObjectKey, thumbReader, int64(len(thumbnailBytes)), "image/webp"); err != nil {
			return model.ServerPostResponse{}, err
		}

		if err = tx.Commit(ctxContext); err != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
			return model.ServerPostResponse{}, err
		}
		committed = true
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err := usecase.PostRepository.GetPost(ctxContext, postId, userId, minioFullUrl)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	return response, nil
}

func (usecase *PostUsecase) GetServerPosts(ctx fiber.Ctx, serverId string, userId string) (model.ServerPostListResponse, error) {
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
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetServerPosts")
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

	var cursor model.ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	posts, err := usecase.PostRepository.GetServerPosts(ctxContext, limit+1, serverId, userId, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	response := model.ServerPostListResponse{Data: []model.ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (usecase *PostUsecase) SearchServerPosts(ctx fiber.Ctx, serverId string, userId string) (model.ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")
	query := strings.TrimSpace(ctx.Query("q", ""))

	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(constant.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostListResponse{}, valErr
	}

	queryLen := len([]rune(query))
	if queryLen < constant.MIN_SEARCH_QUERY_LENGTH {
		return model.ServerPostListResponse{}, &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: fmt.Sprintf("Search query must be at least %d characters", constant.MIN_SEARCH_QUERY_LENGTH),
			Param:   "q",
		}
	}
	if queryLen > constant.MAX_SEARCH_QUERY_LENGTH {
		return model.ServerPostListResponse{}, &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: fmt.Sprintf("Search query must be at most %d characters", constant.MAX_SEARCH_QUERY_LENGTH),
			Param:   "q",
		}
	}

	// Escape ILIKE metacharacters so the user's input matches literally (the SQL
	// uses ESCAPE '\'). Backslash must be escaped first to avoid double-escaping.
	query = strings.ReplaceAll(query, `\`, `\\`)
	query = strings.ReplaceAll(query, `%`, `\%`)
	query = strings.ReplaceAll(query, `_`, `\_`)

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.SearchServerPosts")
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

	var cursor model.ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	posts, err := usecase.PostRepository.SearchServerPosts(ctxContext, limit+1, serverId, userId, query, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	response := model.ServerPostListResponse{Data: []model.ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (usecase *PostUsecase) GetServerPostForMe(ctx fiber.Ctx, serverId string, userId string) (model.ServerPostListResponse, error) {
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
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetServerPostForMe")
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

	var cursor model.ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	posts, err := usecase.PostRepository.GetServerPostForMe(ctxContext, limit+1, serverId, userId, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	response := model.ServerPostListResponse{Data: []model.ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

// GetServerPostsByUserId returns another member's posts in a server (their
// profile grid). The requester must be a member; ownership is computed
// relative to the requester. Reuses the author-filtered repository query.
func (usecase *PostUsecase) GetServerPostsByUserId(ctx fiber.Ctx, requesterUserId, serverId, targetUserId string) (model.ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("userId", targetUserId)
	v.Int("limit", limit).Min(0).Max(constant.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetServerPostsByUserId")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", requesterUserId),
		attribute.String("target.user.id", targetUserId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, requesterUserId)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.ServerPostListResponse{}, err
	}

	var cursor model.ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	posts, err := usecase.PostRepository.GetServerPostsByAuthor(ctxContext, limit+1, serverId, targetUserId, requesterUserId, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	response := model.ServerPostListResponse{Data: []model.ServerPostResponse{}}
	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (usecase *PostUsecase) GetPost(ctx fiber.Ctx, postId string, userId string) (model.ServerPostResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postId)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetPost")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postId),
	)

	serverId, err := usecase.PostRepository.GetPostServerId(ctxContext, postId)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.ServerPostResponse{}, err
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err := usecase.PostRepository.GetPost(ctxContext, postId, userId, minioFullUrl)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	return response, nil
}

func (usecase *PostUsecase) UpdatePostCaption(ctx fiber.Ctx, serverId string, postId string, userId string, payload model.ServerPostUpdateCaptionRequest) (model.ServerPostResponse, error) {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("postId", postId)
	v.String("caption", payload.Caption).Required().MaxLen(2000)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.UpdatePostCaption")
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
		attribute.String("post.id", postId),
	)

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.ServerPostResponse{}, err
	}

	ownerCount, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return model.ServerPostResponse{}, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the author of this post", Param: "postId"}
		return model.ServerPostResponse{}, err
	}

	now := time.Now().UTC()
	err = usecase.PostRepository.UpdatePostCaption(ctxContext, postId, payload.Caption, userId, now)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err := usecase.PostRepository.GetPost(ctxContext, postId, userId, minioFullUrl)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	return response, nil
}

func (usecase *PostUsecase) DeletePost(ctx fiber.Ctx, serverId string, postId string, userId string) error {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("postId", postId)
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.DeletePost")
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
		attribute.String("post.id", postId),
	)

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	ownerCount, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		// Not the post author: only moderators may delete. An admin can remove
		// posts by regular members (or authors who already left), but never the
		// owner's or another admin's post — mirrors the kick permission model.
		var deleterRole string
		deleterRole, err = usecase.ServerRepository.GetMemberRoleName(ctxContext, serverId, userId)
		if err != nil {
			return err
		}

		switch deleterRole {
		case model.OwnerRole:
			// Owner can delete any post in the server.
		case model.AdminRole:
			var authorId string
			authorId, err = usecase.PostRepository.GetPostAuthorId(ctxContext, postId)
			if err != nil {
				return err
			}
			var authorRole string
			authorRole, err = usecase.ServerRepository.GetMemberRoleName(ctxContext, serverId, authorId)
			if err != nil {
				return err
			}
			if authorRole == model.OwnerRole || authorRole == model.AdminRole {
				err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Admins cannot delete posts by the owner or other admins", Param: "postId"}
				return err
			}
		default:
			err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the author of this post", Param: "postId"}
			return err
		}
	}

	err = usecase.PostRepository.DeletePostHard(ctxContext, postId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *PostUsecase) LikePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostLikeResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostLikeResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.LikePost")
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
		return model.PostLikeResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.PostLikeResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.PostLikeResponse{}, err
	}

	now := time.Now().UTC()
	postLike := model.ServerPostLike{
		Id:        uuid.New().String(),
		PostId:    postIdParam,
		UserId:    userId,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	inserted, err := usecase.PostRepository.CreatePostLikeIdempotent(ctxContext, postLike)
	if err != nil {
		return model.PostLikeResponse{}, err
	}

	// Notify only on a NEW like (inserted=true) — repeated like/unlike/like must not re-notify.
	// Notif is best-effort: any resolution error logs + is skipped, never fails the like.
	if inserted {
		postAuthorId, authorErr := usecase.PostRepository.GetPostAuthorId(ctxContext, postIdParam)
		if authorErr != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
		} else if postAuthorId != userId { // self-notif guard
			actorProfileId, found, profErr := usecase.ProfileRepository.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
			if profErr != nil || !found {
				util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
			} else {
				usecase.NotificationUsecase.Notify(ctxContext, []model.NotificationEvent{{
					Type:            "like",
					RecipientUserId: postAuthorId,
					ActorUserId:     userId,
					ActorProfileId:  actorProfileId,
					ServerId:        serverId,
					PostId:          postIdParam,
				}})
			}
		}
	}

	likeCount, err := usecase.PostRepository.GetPostLikeCount(ctxContext, postIdParam)
	if err != nil {
		return model.PostLikeResponse{}, err
	}

	return model.PostLikeResponse{PostId: postIdParam, UserLiked: true, LikeCount: likeCount}, nil
}

func (usecase *PostUsecase) UnlikePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostLikeResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostLikeResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.UnlikePost")
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
		return model.PostLikeResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.PostLikeResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.PostLikeResponse{}, err
	}

	likeExists, err := usecase.PostRepository.CheckPostLike(ctxContext, postIdParam, userId)
	if err != nil {
		return model.PostLikeResponse{}, err
	}

	if likeExists {
		err = usecase.PostRepository.DeletePostLike(ctxContext, postIdParam, userId)
		if err != nil {
			return model.PostLikeResponse{}, err
		}
	}

	likeCount, err := usecase.PostRepository.GetPostLikeCount(ctxContext, postIdParam)
	if err != nil {
		return model.PostLikeResponse{}, err
	}

	return model.PostLikeResponse{PostId: postIdParam, UserLiked: false, LikeCount: likeCount}, nil
}

func (usecase *PostUsecase) CreateComment(ctx fiber.Ctx, postIdParam string, userId string, payload model.ServerCommentCreateRequest) (model.ServerCommentResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	v.String("content", payload.Content).Required().MaxLen(1000)
	if payload.ParentId != nil {
		v.UUID("parentId", *payload.ParentId)
	}
	if valErr := v.Validate(); valErr != nil {
		return model.ServerCommentResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.CreateComment")
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
		return model.ServerCommentResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerCommentResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.ServerCommentResponse{}, err
	}

	if payload.ParentId != nil {
		parentExists, checkErr := usecase.PostRepository.CheckCommentExists(ctxContext, *payload.ParentId, postIdParam)
		if checkErr != nil {
			err = checkErr
			return model.ServerCommentResponse{}, err
		}
		if parentExists == 0 {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Parent comment not found in this post", Param: "parentId"}
			return model.ServerCommentResponse{}, err
		}
	}

	now := time.Now().UTC()
	commentId := uuid.New().String()

	comment := model.ServerPostComment{
		Id:        commentId,
		PostId:    postIdParam,
		AuthorId:  userId,
		ParentId:  payload.ParentId,
		Content:   payload.Content,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}

	err = usecase.PostRepository.CreateComment(ctxContext, comment)
	if err != nil {
		return model.ServerCommentResponse{}, err
	}

	// Notif is best-effort: resolution errors log + skip, never fail the comment (already saved).
	// NoTx variant: trigger runs after the comment is persisted. found=false → empty id → FK
	// violation, so skip the whole notif if the actor profile is unresolved.
	actorProfileId, found, profErr := usecase.ProfileRepository.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
	if profErr != nil || !found {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
	} else {
		// Slice (not single event) so Fase 2.5 can append mention events from the same comment.
		var notifEvents []model.NotificationEvent
		if payload.ParentId == nil {
			// Top-level comment → notify the POST author. Self-notif guard below.
			postAuthorId, authorErr := usecase.PostRepository.GetPostAuthorId(ctxContext, postIdParam)
			if authorErr != nil {
				util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
			} else if postAuthorId != userId {
				notifEvents = append(notifEvents, model.NotificationEvent{
					Type:            "comment",
					RecipientUserId: postAuthorId,
					ActorUserId:     userId,
					ActorProfileId:  actorProfileId,
					ServerId:        serverId,
					PostId:          postIdParam,
				})
			}
		} else {
			// Reply → notify the PARENT COMMENT author, NOT the post owner. Self-notif guard below.
			parentAuthorId, parentErr := usecase.PostRepository.GetCommentAuthorId(ctxContext, *payload.ParentId)
			if parentErr != nil {
				util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve parent comment author", zap.Error(parentErr))
			} else if parentAuthorId != userId {
				notifEvents = append(notifEvents, model.NotificationEvent{
					Type:            "reply",
					RecipientUserId: parentAuthorId,
					ActorUserId:     userId,
					ActorProfileId:  actorProfileId,
					ServerId:        serverId,
					PostId:          postIdParam,
					CommentId:       &commentId,
				})
			}
		}

		if len(notifEvents) > 0 {
			usecase.NotificationUsecase.Notify(ctxContext, notifEvents)
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err := usecase.PostRepository.GetCommentById(ctxContext, commentId, userId, minioFullUrl)
	if err != nil {
		return model.ServerCommentResponse{}, err
	}

	return response, nil
}

func (usecase *PostUsecase) GetComments(ctx fiber.Ctx, postIdParam string, userId string, cursorStr string, limitStr string) (model.ServerCommentListResponse, error) {
	limit := constant.DEFAULT_LIMIT
	if limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil {
			return model.ServerCommentListResponse{}, &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid limit value", Param: "limit"}
		}
		limit = parsed
	}

	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	v.Int("limit", limit).Min(0).Max(constant.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return model.ServerCommentListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetComments")
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
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	serverId, err := usecase.PostRepository.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return model.ServerCommentListResponse{}, err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.ServerCommentListResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return model.ServerCommentListResponse{}, err
	}

	var cursor *model.ServerCommentCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.ServerCommentCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.ServerCommentListResponse{}, err
		}
		cursor = decoded
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	comments, err := usecase.PostRepository.GetComments(ctxContext, limit+1, postIdParam, userId, cursor, minioFullUrl)
	if err != nil {
		return model.ServerCommentListResponse{}, err
	}

	response := model.ServerCommentListResponse{Data: []model.ServerCommentResponse{}}

	if len(comments) > limit {
		response.Data = comments[:limit]
		last := comments[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerCommentCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(comments) > 0 {
		response.Data = comments
	}

	return response, nil
}

func (usecase *PostUsecase) DeleteComment(ctx fiber.Ctx, postIdParam string, commentIdParam string, userId string) error {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	v.UUID("commentId", commentIdParam)
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.DeleteComment")
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
		attribute.String("comment.id", commentIdParam),
	)

	serverId, err := usecase.PostRepository.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return err
	}

	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return err
	}

	ownerCount, err := usecase.PostRepository.CheckCommentOwnership(ctxContext, commentIdParam, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		var role string
		role, err = usecase.ServerRepository.GetMemberRoleName(ctxContext, serverId, userId)
		if err != nil {
			return err
		}
		if role != model.OwnerRole && role != model.AdminRole {
			err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the author of this comment", Param: "commentId"}
			return err
		}
	}

	commentExists, err := usecase.PostRepository.CheckCommentExists(ctxContext, commentIdParam, postIdParam)
	if err != nil {
		return err
	}
	if commentExists == 0 {
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Comment not found in this post", Param: "commentId"}
		return err
	}

	err = usecase.PostRepository.DeleteCommentHard(ctxContext, commentIdParam)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *PostUsecase) SavePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostSaveResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.SavePost")
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

func (usecase *PostUsecase) UnsavePost(ctx fiber.Ctx, postIdParam string, userId string) (model.PostSaveResponse, error) {
	v := util.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return model.PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.UnsavePost")
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
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetSavedPosts")
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

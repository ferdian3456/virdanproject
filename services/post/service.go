package post

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/services/notification"
	"github.com/ferdian3456/virdanproject/services/payment"
	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type Service struct {
	Repo            *Repository
	ServerRepo      *server.Repository
	PaymentRepo     *payment.Repository
	NotificationSvc *notification.Service
	DB              *pgxpool.Pool
	Log             *zap.Logger
	Config          *koanf.Koanf
}

func NewService(repo *Repository, serverRepo *server.Repository, paymentRepo *payment.Repository, notificationSvc *notification.Service, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf) *Service {
	return &Service{
		Repo:            repo,
		ServerRepo:      serverRepo,
		PaymentRepo:     paymentRepo,
		NotificationSvc: notificationSvc,
		DB:              db,
		Log:             log,
		Config:          config,
	}
}

func (service *Service) minioFullUrl() string {
	return fmt.Sprintf("%s%s/%s", service.Config.String("MINIO_HTTP"), service.Config.String("MINIO_URL"), service.Config.String("MINIO_BUCKET_NAME"))
}

func (service *Service) CreatePost(ctx fiber.Ctx, serverId string, userId string) (ServerPostResponse, error) {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.CreatePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostResponse{}, err
	}

	plusActive, _, plusErr := service.PaymentRepo.GetActivePlus(ctxContext, serverId)
	if plusErr != nil {
		err = plusErr
		return ServerPostResponse{}, err
	}
	maxImageSize := int64(shared.MAX_IMAGE_SIZE_FREE)
	maxVideoSize := int64(shared.MAX_VIDEO_SIZE_FREE)
	if plusActive {
		maxImageSize = int64(shared.MAX_IMAGE_SIZE_PLUS)
		maxVideoSize = int64(shared.MAX_VIDEO_SIZE_PLUS)
	}

	imageHeader, imageErr := ctx.FormFile("image")
	hasImage := imageErr == nil && imageHeader != nil
	videoHeader, videoErr := ctx.FormFile("video")
	hasVideo := videoErr == nil && videoHeader != nil

	if !hasImage && !hasVideo {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "image or video is required", Param: "image"}
		return ServerPostResponse{}, err
	}
	if hasImage && hasVideo {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "provide either image or video, not both", Param: "image"}
		return ServerPostResponse{}, err
	}

	caption := ctx.FormValue("caption")
	mirrorVideo := ctx.FormValue("mirror") == "true"
	v.Reset()
	v.String("caption", caption).Required().MaxLen(2000)
	if valErr := v.Validate(); valErr != nil {
		err = valErr
		return ServerPostResponse{}, err
	}

	now := time.Now().UTC()
	postId := uuid.New().String()
	bucketName := service.Config.String("MINIO_BUCKET_NAME")

	if hasImage {
		imageFile, imageSize, imageWidth, imageHeight, imgErr := shared.ValidateImage(ctxContext, imageHeader, "image", maxImageSize, 1080, 1440, false)
		if imgErr != nil {
			err = imgErr
			return ServerPostResponse{}, err
		}

		postImageId := uuid.New().String()
		objectKey := fmt.Sprintf("server/post/%s.webp", postImageId)

		serverPostImage := ServerPostImage{
			Id:        postImageId,
			Bucket:    bucketName,
			ObjectKey: objectKey,
			MimeType:  "image/webp",
			Size:      imageSize,
			Width:     imageWidth,
			Height:    imageHeight,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		serverPost := ServerPost{
			Id: postId, ServerId: serverId, AuthorId: userId,
			PostImageId: &postImageId, Caption: caption,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		committed := false
		tx, txErr := service.DB.Begin(ctxContext)
		if txErr != nil {
			err = txErr
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
			return ServerPostResponse{}, err
		}
		defer func() {
			if !committed {
				_ = tx.Rollback(ctxContext)
			}
		}()

		if err = service.Repo.CreateServerPostImage(ctxContext, tx, serverPostImage); err != nil {
			return ServerPostResponse{}, err
		}
		if err = service.Repo.CreateServerPost(ctxContext, tx, serverPost); err != nil {
			return ServerPostResponse{}, err
		}
		if err = service.Repo.UploadPostObject(ctxContext, bucketName, objectKey, imageFile, imageSize, "image/webp"); err != nil {
			return ServerPostResponse{}, err
		}
		if err = tx.Commit(ctxContext); err != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
			return ServerPostResponse{}, err
		}
		committed = true

	} else {
		if vErr := shared.ValidateVideoFile(ctxContext, videoHeader, "video", maxVideoSize); vErr != nil {
			err = vErr
			return ServerPostResponse{}, err
		}

		tmpDir := service.Config.String("VIDEO_TMP_DIR")
		if tmpDir == "" {
			tmpDir = os.TempDir()
		}
		tmpPath, tmpFile, saveErr := shared.SaveMultipartToTemp(videoHeader, tmpDir, "vid-"+postId)
		if saveErr != nil {
			err = saveErr
			return ServerPostResponse{}, err
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}()

		duration, videoWidth, videoHeight, probeErr := shared.ProbeVideoMetadata(ctxContext, tmpPath)
		if probeErr != nil {
			err = probeErr
			return ServerPostResponse{}, err
		}

		if duration > shared.MAX_VIDEO_DURATION {
			err = &shared.BadRequestError{
				Code:    shared.ERR_VALIDATION_CODE,
				Message: "video duration exceeded " + strconv.Itoa(shared.MAX_VIDEO_DURATION) + "s limit",
				Param:   "video",
			}
			return ServerPostResponse{}, err
		}

		seekSec := min(1.0, float64(duration)/10.0)
		thumbnailBytes, thumbErr := shared.GenerateVideoThumbnail(ctxContext, tmpPath, 75, seekSec)
		if thumbErr != nil {
			err = thumbErr
			return ServerPostResponse{}, err
		}

		postVideoId := uuid.New().String()
		ext := filepath.Ext(videoHeader.Filename)
		videoObjectKey := fmt.Sprintf("server/post/%s%s", postVideoId, ext)
		thumbObjectKey := fmt.Sprintf("server/post/%s_thumb.webp", postVideoId)

		mimeType := "video/mp4"
		switch strings.ToLower(ext) {
		case ".mov":
			mimeType = "video/quicktime"
		case ".webm":
			mimeType = "video/webm"
		}

		serverPostVideo := ServerPostVideo{
			Id: postVideoId, Bucket: bucketName, ObjectKey: videoObjectKey,
			MimeType: mimeType, Size: videoHeader.Size,
			Duration: duration, Width: videoWidth, Height: videoHeight,
			ThumbnailObjectKey: thumbObjectKey,
			Mirrored:           mirrorVideo,
			CreatedAt:          now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		serverPost := ServerPost{
			Id: postId, ServerId: serverId, AuthorId: userId,
			PostVideoId: &postVideoId, Caption: caption,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}

		committed := false
		tx, txErr := service.DB.Begin(ctxContext)
		if txErr != nil {
			err = txErr
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
			return ServerPostResponse{}, err
		}
		defer func() {
			if !committed {
				_ = tx.Rollback(ctxContext)
			}
		}()

		if err = service.Repo.CreateServerPostVideo(ctxContext, tx, serverPostVideo); err != nil {
			return ServerPostResponse{}, err
		}
		if err = service.Repo.CreateServerPost(ctxContext, tx, serverPost); err != nil {
			return ServerPostResponse{}, err
		}

		videoFile, openErr := os.Open(tmpPath)
		if openErr != nil {
			err = openErr
			return ServerPostResponse{}, err
		}
		defer func() { _ = videoFile.Close() }()

		videoBytes, readErr := io.ReadAll(videoFile)
		if readErr != nil {
			err = readErr
			return ServerPostResponse{}, err
		}
		videoReader := bytes.NewReader(videoBytes)

		if err = service.Repo.UploadPostObject(ctxContext, bucketName, videoObjectKey, videoReader, int64(len(videoBytes)), mimeType); err != nil {
			return ServerPostResponse{}, err
		}

		thumbReader := bytes.NewReader(thumbnailBytes)
		if err = service.Repo.UploadPostObject(ctxContext, bucketName, thumbObjectKey, thumbReader, int64(len(thumbnailBytes)), "image/webp"); err != nil {
			return ServerPostResponse{}, err
		}

		if err = tx.Commit(ctxContext); err != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
			return ServerPostResponse{}, err
		}
		committed = true
	}

	response, err := service.Repo.GetPost(ctxContext, postId, userId, service.minioFullUrl())
	if err != nil {
		return ServerPostResponse{}, err
	}

	return response, nil
}

func (service *Service) GetServerPosts(ctx fiber.Ctx, serverId string, userId string) (ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", shared.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerPosts")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostListResponse{}, err
	}

	var cursor ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	posts, err := service.Repo.GetServerPosts(ctxContext, limit+1, serverId, userId, &cursor, service.minioFullUrl())
	if err != nil {
		return ServerPostListResponse{}, err
	}

	response := ServerPostListResponse{Data: []ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (service *Service) SearchServerPosts(ctx fiber.Ctx, serverId string, userId string) (ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", shared.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")
	query := strings.TrimSpace(ctx.Query("q", ""))

	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostListResponse{}, valErr
	}

	queryLen := len([]rune(query))
	if queryLen < shared.MIN_SEARCH_QUERY_LENGTH {
		return ServerPostListResponse{}, &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: fmt.Sprintf("Search query must be at least %d characters", shared.MIN_SEARCH_QUERY_LENGTH),
			Param:   "q",
		}
	}
	if queryLen > shared.MAX_SEARCH_QUERY_LENGTH {
		return ServerPostListResponse{}, &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: fmt.Sprintf("Search query must be at most %d characters", shared.MAX_SEARCH_QUERY_LENGTH),
			Param:   "q",
		}
	}

	query = strings.ReplaceAll(query, `\`, `\\`)
	query = strings.ReplaceAll(query, `%`, `\%`)
	query = strings.ReplaceAll(query, `_`, `\_`)

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.SearchServerPosts")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostListResponse{}, err
	}

	var cursor ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	posts, err := service.Repo.SearchServerPosts(ctxContext, limit+1, serverId, userId, query, &cursor, service.minioFullUrl())
	if err != nil {
		return ServerPostListResponse{}, err
	}

	response := ServerPostListResponse{Data: []ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (service *Service) GetServerPostForMe(ctx fiber.Ctx, serverId string, userId string) (ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", shared.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerPostForMe")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostListResponse{}, err
	}

	var cursor ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	posts, err := service.Repo.GetServerPostForMe(ctxContext, limit+1, serverId, userId, &cursor, service.minioFullUrl())
	if err != nil {
		return ServerPostListResponse{}, err
	}

	response := ServerPostListResponse{Data: []ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (service *Service) GetServerPostsByUserId(ctx fiber.Ctx, requesterUserId, serverId, targetUserId string) (ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", shared.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("userId", targetUserId)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerPostsByUserId")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
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

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, requesterUserId)
	if err != nil {
		return ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostListResponse{}, err
	}

	var cursor ServerPostCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[ServerPostCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	posts, err := service.Repo.GetServerPostsByAuthor(ctxContext, limit+1, serverId, targetUserId, requesterUserId, &cursor, service.minioFullUrl())
	if err != nil {
		return ServerPostListResponse{}, err
	}

	response := ServerPostListResponse{Data: []ServerPostResponse{}}
	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerPostCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

func (service *Service) GetPost(ctx fiber.Ctx, postId string, userId string) (ServerPostResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postId)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetPost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postId),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postId)
	if err != nil {
		return ServerPostResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return ServerPostResponse{}, err
	}

	response, err := service.Repo.GetPost(ctxContext, postId, userId, service.minioFullUrl())
	if err != nil {
		return ServerPostResponse{}, err
	}

	return response, nil
}

func (service *Service) UpdatePostCaption(ctx fiber.Ctx, serverId string, postId string, userId string, payload ServerPostUpdateCaptionRequest) (ServerPostResponse, error) {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("postId", postId)
	v.String("caption", payload.Caption).Required().MaxLen(2000)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdatePostCaption")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("post.id", postId),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostResponse{}, err
	}

	ownerCount, err := service.Repo.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return ServerPostResponse{}, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the author of this post", Param: "postId"}
		return ServerPostResponse{}, err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdatePostCaption(ctxContext, postId, payload.Caption, userId, now)
	if err != nil {
		return ServerPostResponse{}, err
	}

	response, err := service.Repo.GetPost(ctxContext, postId, userId, service.minioFullUrl())
	if err != nil {
		return ServerPostResponse{}, err
	}

	return response, nil
}

func (service *Service) DeletePost(ctx fiber.Ctx, serverId string, postId string, userId string) error {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("postId", postId)
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.DeletePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("post.id", postId),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	ownerCount, err := service.Repo.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		var deleterRole string
		deleterRole, err = service.ServerRepo.GetMemberRoleName(ctxContext, serverId, userId)
		if err != nil {
			return err
		}

		switch deleterRole {
		case server.OwnerRole:
		case server.AdminRole:
			var authorId string
			authorId, err = service.Repo.GetPostAuthorId(ctxContext, postId)
			if err != nil {
				return err
			}
			var authorRole string
			authorRole, err = service.ServerRepo.GetMemberRoleName(ctxContext, serverId, authorId)
			if err != nil {
				return err
			}
			if authorRole == server.OwnerRole || authorRole == server.AdminRole {
				err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Admins cannot delete posts by the owner or other admins", Param: "postId"}
				return err
			}
		default:
			err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the author of this post", Param: "postId"}
			return err
		}
	}

	err = service.Repo.DeletePostHard(ctxContext, postId)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) LikePost(ctx fiber.Ctx, postIdParam string, userId string) (PostLikeResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return PostLikeResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.LikePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return PostLikeResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return PostLikeResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return PostLikeResponse{}, err
	}

	now := time.Now().UTC()
	postLike := ServerPostLike{
		Id:        uuid.New().String(),
		PostId:    postIdParam,
		UserId:    userId,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	inserted, err := service.Repo.CreatePostLikeIdempotent(ctxContext, postLike)
	if err != nil {
		return PostLikeResponse{}, err
	}

	if inserted {
		postAuthorId, authorErr := service.Repo.GetPostAuthorId(ctxContext, postIdParam)
		if authorErr != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
		} else if postAuthorId != userId {
			actorProfileId, found, profErr := service.ServerRepo.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
			if profErr != nil || !found {
				shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
			} else {
				service.NotificationSvc.Notify(ctxContext, []notification.NotificationEvent{{
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

	likeCount, err := service.Repo.GetPostLikeCount(ctxContext, postIdParam)
	if err != nil {
		return PostLikeResponse{}, err
	}

	return PostLikeResponse{PostId: postIdParam, UserLiked: true, LikeCount: likeCount}, nil
}

func (service *Service) UnlikePost(ctx fiber.Ctx, postIdParam string, userId string) (PostLikeResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return PostLikeResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UnlikePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return PostLikeResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return PostLikeResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return PostLikeResponse{}, err
	}

	likeExists, err := service.Repo.CheckPostLike(ctxContext, postIdParam, userId)
	if err != nil {
		return PostLikeResponse{}, err
	}

	if likeExists {
		err = service.Repo.DeletePostLike(ctxContext, postIdParam, userId)
		if err != nil {
			return PostLikeResponse{}, err
		}
	}

	likeCount, err := service.Repo.GetPostLikeCount(ctxContext, postIdParam)
	if err != nil {
		return PostLikeResponse{}, err
	}

	return PostLikeResponse{PostId: postIdParam, UserLiked: false, LikeCount: likeCount}, nil
}

func (service *Service) CreateComment(ctx fiber.Ctx, postIdParam string, userId string, payload ServerCommentCreateRequest) (ServerCommentResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	v.String("content", payload.Content).Required().MaxLen(1000)
	if payload.ParentId != nil {
		v.UUID("parentId", *payload.ParentId)
	}
	if valErr := v.Validate(); valErr != nil {
		return ServerCommentResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.CreateComment")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return ServerCommentResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerCommentResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return ServerCommentResponse{}, err
	}

	if payload.ParentId != nil {
		parentExists, checkErr := service.Repo.CheckCommentExists(ctxContext, *payload.ParentId, postIdParam)
		if checkErr != nil {
			err = checkErr
			return ServerCommentResponse{}, err
		}
		if parentExists == 0 {
			err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Parent comment not found in this post", Param: "parentId"}
			return ServerCommentResponse{}, err
		}
	}

	now := time.Now().UTC()
	commentId := uuid.New().String()

	comment := ServerPostComment{
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

	err = service.Repo.CreateComment(ctxContext, comment)
	if err != nil {
		return ServerCommentResponse{}, err
	}

	actorProfileId, found, profErr := service.ServerRepo.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
	if profErr != nil || !found {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
	} else {
		var notifEvents []notification.NotificationEvent
		if payload.ParentId == nil {
			postAuthorId, authorErr := service.Repo.GetPostAuthorId(ctxContext, postIdParam)
			if authorErr != nil {
				shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
			} else if postAuthorId != userId {
				notifEvents = append(notifEvents, notification.NotificationEvent{
					Type:            "comment",
					RecipientUserId: postAuthorId,
					ActorUserId:     userId,
					ActorProfileId:  actorProfileId,
					ServerId:        serverId,
					PostId:          postIdParam,
				})
			}
		} else {
			parentAuthorId, parentErr := service.Repo.GetCommentAuthorId(ctxContext, *payload.ParentId)
			if parentErr != nil {
				shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("notif skipped: failed to resolve parent comment author", zap.Error(parentErr))
			} else if parentAuthorId != userId {
				notifEvents = append(notifEvents, notification.NotificationEvent{
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
			service.NotificationSvc.Notify(ctxContext, notifEvents)
		}
	}

	response, err := service.Repo.GetCommentById(ctxContext, commentId, userId, service.minioFullUrl())
	if err != nil {
		return ServerCommentResponse{}, err
	}

	return response, nil
}

func (service *Service) GetComments(ctx fiber.Ctx, postIdParam string, userId string, cursorStr string, limitStr string) (ServerCommentListResponse, error) {
	limit := shared.DEFAULT_LIMIT
	if limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil {
			return ServerCommentListResponse{}, &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid limit value", Param: "limit"}
		}
		limit = parsed
	}

	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerCommentListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetComments")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return ServerCommentListResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerCommentListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return ServerCommentListResponse{}, err
	}

	var cursor *ServerCommentCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[ServerCommentCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerCommentListResponse{}, err
		}
		cursor = decoded
	}

	comments, err := service.Repo.GetComments(ctxContext, limit+1, postIdParam, userId, cursor, service.minioFullUrl())
	if err != nil {
		return ServerCommentListResponse{}, err
	}

	response := ServerCommentListResponse{Data: []ServerCommentResponse{}}

	if len(comments) > limit {
		response.Data = comments[:limit]
		last := comments[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerCommentCursor{
			Id:        last.Id,
			CreatedAt: last.CreatedAt,
		})
	} else if len(comments) > 0 {
		response.Data = comments
	}

	return response, nil
}

func (service *Service) DeleteComment(ctx fiber.Ctx, postIdParam string, commentIdParam string, userId string) error {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	v.UUID("commentId", commentIdParam)
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.DeleteComment")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
		attribute.String("comment.id", commentIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return err
	}

	ownerCount, err := service.Repo.CheckCommentOwnership(ctxContext, commentIdParam, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		var role string
		role, err = service.ServerRepo.GetMemberRoleName(ctxContext, serverId, userId)
		if err != nil {
			return err
		}
		if role != server.OwnerRole && role != server.AdminRole {
			err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the author of this comment", Param: "commentId"}
			return err
		}
	}

	commentExists, err := service.Repo.CheckCommentExists(ctxContext, commentIdParam, postIdParam)
	if err != nil {
		return err
	}
	if commentExists == 0 {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Comment not found in this post", Param: "commentId"}
		return err
	}

	err = service.Repo.DeleteCommentHard(ctxContext, commentIdParam)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) SavePost(ctx fiber.Ctx, postIdParam string, userId string) (PostSaveResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.SavePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return PostSaveResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return PostSaveResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return PostSaveResponse{}, err
	}

	saveExists, err := service.Repo.CheckPostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return PostSaveResponse{}, err
	}
	if saveExists {
		err = &shared.ConflictError{Code: shared.ERR_CONFLICT_CODE, Message: "Post sudah disimpan", Param: "postId"}
		return PostSaveResponse{}, err
	}

	now := time.Now().UTC()
	postSave := ServerPostSave{
		Id:        uuid.New().String(),
		PostId:    postIdParam,
		UserId:    userId,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	err = service.Repo.CreatePostSave(ctxContext, postSave)
	if err != nil {
		return PostSaveResponse{}, err
	}

	return PostSaveResponse{PostId: postIdParam, UserSaved: true}, nil
}

func (service *Service) UnsavePost(ctx fiber.Ctx, postIdParam string, userId string) (PostSaveResponse, error) {
	v := shared.NewValidator()
	v.UUID("postId", postIdParam)
	if valErr := v.Validate(); valErr != nil {
		return PostSaveResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UnsavePost")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("post.id", postIdParam),
	)

	serverId, err := service.Repo.GetPostServerId(ctxContext, postIdParam)
	if err != nil {
		return PostSaveResponse{}, err
	}

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return PostSaveResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "postId"}
		return PostSaveResponse{}, err
	}

	saveExists, err := service.Repo.CheckPostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return PostSaveResponse{}, err
	}
	if !saveExists {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Post belum disimpan", Param: "postId"}
		return PostSaveResponse{}, err
	}

	err = service.Repo.DeletePostSave(ctxContext, postIdParam, userId)
	if err != nil {
		return PostSaveResponse{}, err
	}

	return PostSaveResponse{PostId: postIdParam, UserSaved: false}, nil
}

func (service *Service) GetSavedPosts(ctx fiber.Ctx, serverId string, userId string) (ServerPostListResponse, error) {
	limit := fiber.Query[int](ctx, "limit", shared.DEFAULT_LIMIT)
	cursorStr := ctx.Query("cursor", "")

	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.Int("limit", limit).Min(0).Max(shared.MAX_LIMIT)
	if valErr := v.Validate(); valErr != nil {
		return ServerPostListResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetSavedPosts")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursorStr),
	)

	memberCount, err := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return ServerPostListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return ServerPostListResponse{}, err
	}

	var cursor SavedPostCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[SavedPostCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return ServerPostListResponse{}, err
		}
		cursor = *decoded
	}

	posts, err := service.Repo.GetSavedPosts(ctxContext, limit+1, serverId, userId, &cursor, service.minioFullUrl())
	if err != nil {
		return ServerPostListResponse{}, err
	}

	response := ServerPostListResponse{Data: []ServerPostResponse{}}

	if len(posts) > limit {
		response.Data = posts[:limit]
		last := posts[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(SavedPostCursor{
			SavedAt: *last.SavedAt,
			PostId:  last.Id,
		})
	} else if len(posts) > 0 {
		response.Data = posts
	}

	return response, nil
}

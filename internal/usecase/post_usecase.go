package usecase

import (
	"fmt"
	"strconv"
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
	PostRepository      *repository.PostRepository
	ServerRepository    *repository.ServerRepository
	ProfileRepository   *repository.ProfileRepository
	NotificationUsecase *NotificationUsecase
	DB                  *pgxpool.Pool
	Log                 *zap.Logger
	Config              *koanf.Koanf
}

func NewPostUsecase(
	postRepository *repository.PostRepository,
	serverRepository *repository.ServerRepository,
	profileRepository *repository.ProfileRepository,
	notificationUsecase *NotificationUsecase,
	db *pgxpool.Pool,
	zap *zap.Logger,
	koanf *koanf.Koanf,
) *PostUsecase {
	return &PostUsecase{
		PostRepository:      postRepository,
		ServerRepository:    serverRepository,
		ProfileRepository:   profileRepository,
		NotificationUsecase: notificationUsecase,
		DB:                  db,
		Log:                 zap,
		Config:              koanf,
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.CreatePost")
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

	imageFile, imageSize, err := util.ExtractAndValidateImage(ctx, ctxContext, "image")
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	caption := ctx.FormValue("caption")
	v.Reset()
	v.String("caption", caption).Required().MaxLen(2000)
	if valErr := v.Validate(); valErr != nil {
		err = valErr
		return model.ServerPostResponse{}, err
	}

	now := time.Now().UTC()
	postImageId := uuid.New().String()
	postId := uuid.New().String()
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")
	objectKey := fmt.Sprintf("server/post/%s.webp", postImageId)

	serverPostImage := model.ServerPostImage{
		Id:        postImageId,
		Bucket:    bucketName,
		ObjectKey: objectKey,
		MimeType:  "image/webp",
		Size:      imageSize,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}

	serverPost := model.ServerPost{
		Id:          postId,
		ServerId:    serverId,
		AuthorId:    userId,
		PostImageId: &postImageId,
		Caption:     caption,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}

	committed := false
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return model.ServerPostResponse{}, err
	}
	defer func() {
		if !committed {
			_ = tx.Rollback(ctxContext)
		}
	}()

	err = usecase.PostRepository.CreateServerPostImage(ctxContext, tx, serverPostImage)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	err = usecase.PostRepository.CreateServerPost(ctxContext, tx, serverPost)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, objectKey, imageFile, imageSize)
	if err != nil {
		return model.ServerPostResponse{}, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return model.ServerPostResponse{}, err
	}
	committed = true

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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerPosts")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerPostForMe")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerPostsByUserId")
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

	posts, err := usecase.PostRepository.GetServerPostForMe(ctxContext, limit+1, serverId, targetUserId, &cursor, minioFullUrl)
	if err != nil {
		return model.ServerPostListResponse{}, err
	}

	// Ownership is relative to the requester, not the post author.
	isOwner := requesterUserId == targetUserId
	for i := range posts {
		posts[i].IsOwner = isOwner
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetPost")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdatePostCaption")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.DeletePost")
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
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the author of this post", Param: "postId"}
		return err
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.LikePost")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UnlikePost")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.CreateComment")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetComments")
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
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.DeleteComment")
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
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the author of this comment", Param: "commentId"}
		return err
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


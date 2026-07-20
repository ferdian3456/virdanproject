package post

import (
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type Controller struct {
	Service *Service
	Log     *zap.Logger
	Config  *koanf.Koanf
}

func NewController(service *Service, log *zap.Logger, config *koanf.Koanf) *Controller {
	return &Controller{
		Service: service,
		Log:     log,
		Config:  config,
	}
}

// CreatePost godoc
// @Summary Create a post in a server (image or video)
// @description.markdown create_post
// @Tags posts
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param caption formData string true "Post caption"
// @Param image formData file false "Image file (mutually exclusive with video)"
// @Param video formData file false "Video file, max 180s (mutually exclusive with image)"
// @Param mirror formData bool false "Mirror the video horizontally"
// @Success 200 {object} post.ServerPostResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts [post]
func (controller *Controller) CreatePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.CreatePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerPostResponse
	response, err = controller.Service.CreatePost(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdatePost godoc
// @Summary Update a post's caption
// @description.markdown update_post
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param postId path string true "Post ID (UUID)"
// @Param request body post.ServerPostUpdateCaptionRequest true "New caption"
// @Success 200 {object} post.ServerPostResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts/{postId} [put]
func (controller *Controller) UpdatePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdatePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	postId := ctx.Params("postId")

	var payload ServerPostUpdateCaptionRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerPostResponse
	response, err = controller.Service.UpdatePostCaption(ctx, serverId, postId, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerPosts godoc
// @Summary List posts in a server
// @description.markdown get_server_posts
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerPostListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts [get]
func (controller *Controller) GetServerPosts(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerPosts")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerPostListResponse
	response, err = controller.Service.GetServerPosts(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// SearchServerPosts godoc
// @Summary Search posts in a server by caption
// @description.markdown search_server_posts
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param q query string true "Search query (2-100 chars)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerPostListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts/search [get]
func (controller *Controller) SearchServerPosts(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.SearchServerPosts")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerPostListResponse
	response, err = controller.Service.SearchServerPosts(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerPostForMe godoc
// @Summary List the logged-in user's own posts in a server
// @description.markdown get_server_post_for_me
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerPostListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts/me [get]
func (controller *Controller) GetServerPostForMe(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerPostForMe")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerPostListResponse
	response, err = controller.Service.GetServerPostForMe(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerPostsByUserId godoc
// @Summary List a specific member's posts in a server
// @description.markdown get_server_posts_by_user
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param userId path string true "Target user ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerPostListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/{userId}/posts [get]
func (controller *Controller) GetServerPostsByUserId(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerPostsByUserId")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	requesterUserId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	var response ServerPostListResponse
	response, err = controller.Service.GetServerPostsByUserId(ctx, requesterUserId, serverId, targetUserId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// DeletePost godoc
// @Summary Delete a post (author, or server owner/admin)
// @description.markdown delete_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts/{postId} [delete]
func (controller *Controller) DeletePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.DeletePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	postId := ctx.Params("postId")

	err = controller.Service.DeletePost(ctx, serverId, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// GetPost godoc
// @Summary Get a single post by ID
// @description.markdown get_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} post.ServerPostResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId} [get]
func (controller *Controller) GetPost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetPost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response ServerPostResponse
	response, err = controller.Service.GetPost(ctx, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// LikePost godoc
// @Summary Like a post (idempotent)
// @description.markdown like_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} post.PostLikeResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/likes [post]
func (controller *Controller) LikePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.LikePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response PostLikeResponse
	response, err = controller.Service.LikePost(ctx, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UnlikePost godoc
// @Summary Unlike a post (idempotent)
// @description.markdown unlike_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} post.PostLikeResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/likes [delete]
func (controller *Controller) UnlikePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UnlikePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response PostLikeResponse
	response, err = controller.Service.UnlikePost(ctx, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// CreateComment godoc
// @Summary Create a comment (or reply) on a post
// @description.markdown create_comment
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Param request body post.ServerCommentCreateRequest true "Comment content and optional parentId"
// @Success 200 {object} post.ServerCommentResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/comments [post]
func (controller *Controller) CreateComment(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.CreateComment")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var payload ServerCommentCreateRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerCommentResponse
	response, err = controller.Service.CreateComment(ctx, postId, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetComments godoc
// @Summary List comments on a post
// @description.markdown get_comments
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerCommentListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/comments [get]
func (controller *Controller) GetComments(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetComments")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")
	cursor := ctx.Query("cursor")
	limit := ctx.Query("limit")

	var response ServerCommentListResponse
	response, err = controller.Service.GetComments(ctx, postId, userId, cursor, limit)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// DeleteComment godoc
// @Summary Delete a comment (author, or server owner/admin)
// @description.markdown delete_comment
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Param commentId path string true "Comment ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/comments/{commentId} [delete]
func (controller *Controller) DeleteComment(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.DeleteComment")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")
	commentId := ctx.Params("commentId")

	err = controller.Service.DeleteComment(ctx, postId, commentId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// SavePost godoc
// @Summary Save (bookmark) a post
// @description.markdown save_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} post.PostSaveResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/saves [post]
func (controller *Controller) SavePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.SavePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response PostSaveResponse
	response, err = controller.Service.SavePost(ctx, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UnsavePost godoc
// @Summary Unsave (remove bookmark from) a post
// @description.markdown unsave_post
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID (UUID)"
// @Success 200 {object} post.PostSaveResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /posts/{postId}/saves [delete]
func (controller *Controller) UnsavePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UnsavePost")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	postId := ctx.Params("postId")

	var response PostSaveResponse
	response, err = controller.Service.UnsavePost(ctx, postId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetSavedPosts godoc
// @Summary List the logged-in user's saved (bookmarked) posts in a server
// @description.markdown get_saved_posts
// @Tags posts
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} post.ServerPostListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/posts/saved [get]
func (controller *Controller) GetSavedPosts(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetSavedPosts")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerPostListResponse
	response, err = controller.Service.GetSavedPosts(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

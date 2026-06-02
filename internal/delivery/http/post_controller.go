package http

import (
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type PostController struct {
	PostUsecase *usecase.PostUsecase
	Log         *zap.Logger
	Config      *koanf.Koanf
}

func NewPostController(postUsecase *usecase.PostUsecase, zap *zap.Logger, koanf *koanf.Koanf) *PostController {
	return &PostController{
		PostUsecase: postUsecase,
		Log:         zap,
		Config:      koanf,
	}
}

// CreatePost godoc
// @Summary      Create a new post
// @Description.markdown create_post
// @Tags         posts
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        image formData file false "Post image file"
// @Param        caption formData string true "Post caption"
// @Success      201   {object}  model.ServerPostResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts [post]
func (controller *PostController) CreatePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.CreatePost")
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

	var response model.ServerPostResponse
	response, err = controller.PostUsecase.CreatePost(ctx, serverId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdatePost godoc
// @Summary      Update post caption
// @Description.markdown update_post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        postId path string true "Post UUID"
// @Param        body body model.ServerPostUpdateCaptionRequest true "Payload"
// @Success      200   {object}  model.ServerPostResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts/{postId} [put]
func (controller *PostController) UpdatePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdatePost")
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
	postId := ctx.Params("postId")

	var payload model.ServerPostUpdateCaptionRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerPostResponse
	response, err = controller.PostUsecase.UpdatePostCaption(ctx, serverId, postId, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetServerPosts godoc
// @Summary      Get posts from a server
// @Description.markdown get_server_posts
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerPostListResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts [get]
func (controller *PostController) GetServerPosts(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerPosts")
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
	response, err = controller.PostUsecase.GetServerPosts(ctx, serverId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetServerPostForMe godoc
// @Summary      Get my posts in a server
// @Description.markdown get_server_post_for_me
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerPostListResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts/me [get]
func (controller *PostController) GetServerPostForMe(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerPostForMe")
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
	response, err = controller.PostUsecase.GetServerPostForMe(ctx, serverId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetServerPostsByUserId godoc
// @Summary      Get another member's posts in a server
// @Description.markdown get_server_posts_by_user
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        userId path string true "Target user UUID"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerPostListResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/members/{userId}/posts [get]
func (controller *PostController) GetServerPostsByUserId(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerPostsByUserId")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	requesterUserId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	var response model.ServerPostListResponse
	response, err = controller.PostUsecase.GetServerPostsByUserId(ctx, requesterUserId, serverId, targetUserId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// DeletePost godoc
// @Summary      Delete a post
// @Description.markdown delete_post
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        postId path string true "Post UUID"
// @Success      200
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /servers/{serverId}/posts/{postId} [delete]
func (controller *PostController) DeletePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.DeletePost")
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
	postId := ctx.Params("postId")

	err = controller.PostUsecase.DeletePost(ctx, serverId, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// GetPost godoc
// @Summary      Get a single post
// @Description.markdown get_post
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.ServerPostResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      404   {object}  model.NotFoundError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId} [get]
func (controller *PostController) GetPost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetPost")
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

	var response model.ServerPostResponse
	response, err = controller.PostUsecase.GetPost(ctx, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// LikePost godoc
// @Summary      Like a post
// @Description.markdown like_post
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.PostLikeResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/likes [post]
func (controller *PostController) LikePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.LikePost")
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

	var response model.PostLikeResponse
	response, err = controller.PostUsecase.LikePost(ctx, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UnlikePost godoc
// @Summary      Unlike a post
// @Description.markdown unlike_post
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.PostLikeResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/likes [delete]
func (controller *PostController) UnlikePost(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UnlikePost")
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

	var response model.PostLikeResponse
	response, err = controller.PostUsecase.UnlikePost(ctx, postId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// CreateComment godoc
// @Summary      Create a comment on a post
// @Description.markdown create_comment
// @Tags         post-interactions
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Param        body body model.ServerCommentCreateRequest true "Payload"
// @Success      201   {object}  model.ServerCommentResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/comments [post]
func (controller *PostController) CreateComment(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.CreateComment")
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

	var payload model.ServerCommentCreateRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerCommentResponse
	response, err = controller.PostUsecase.CreateComment(ctx, postId, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetComments godoc
// @Summary      Get comments on a post
// @Description.markdown get_comments
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerCommentListResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/comments [get]
func (controller *PostController) GetComments(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetComments")
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
	cursor := ctx.Query("cursor")
	limit := ctx.Query("limit")

	var response model.ServerCommentListResponse
	response, err = controller.PostUsecase.GetComments(ctx, postId, userId, cursor, limit)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// DeleteComment godoc
// @Summary      Delete a comment
// @Description.markdown delete_comment
// @Tags         post-interactions
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Param        commentId path string true "Comment UUID"
// @Success      200
// @Failure      400   {object}  model.BadRequestError
// @Failure      403   {object}  model.ForbiddenError
// @Failure      500   {object}  model.BadRequestError
// @Router       /posts/{postId}/comments/{commentId} [delete]
func (controller *PostController) DeleteComment(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.DeleteComment")
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
	commentId := ctx.Params("commentId")

	err = controller.PostUsecase.DeleteComment(ctx, postId, commentId, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

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

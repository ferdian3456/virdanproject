package http

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/knadh/koanf/v2"
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
// @Param        image formData file true "Post image file"
// @Param        caption formData string true "Post caption"
// @Success      200   {object}  model.ServerPostResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/posts [post]
func (controller *PostController) CreatePost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	serverIdParam := ctx.Params("serverId")
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		modelErr := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
		return util.RecordAndSendValidationError(ctx, controller.Log, modelErr, "PostController.CreatePost")
	}

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.CreatePost(ctx, serverId, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.CreatePost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/posts/{postId} [put]
func (controller *PostController) UpdatePost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	serverIdParam := ctx.Params("serverId")
	postIdParam := ctx.Params("postId")

	var payload model.ServerPostUpdateCaptionRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		modelErr := &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}
		return util.RecordAndSendValidationError(ctx, controller.Log, modelErr, "PostController.UpdatePost")
	}

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.UpdatePostCaption(ctx, serverIdParam, postIdParam, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.UpdatePost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/posts/{postId} [delete]
func (controller *PostController) DeletePost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	serverIdParam := ctx.Params("serverId")
	postIdParam := ctx.Params("postId")

	var validationErr *model.ValidationError

	err := controller.PostUsecase.DeletePost(ctx, serverIdParam, postIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.DeletePost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/posts [get]
func (controller *PostController) GetServerPosts(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	serverIdParam := ctx.Params("serverId")

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.GetServerPosts(ctx, serverIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.GetServerPosts")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetPost godoc
// @Summary      Get a single post
// @Description.markdown get_post
// @Tags         posts
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        postId path string true "Post UUID"
// @Success      200   {object}  model.ServerPostResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId} [get]
func (controller *PostController) GetPost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.GetPost(ctx, postIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.GetPost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId}/likes [post]
func (controller *PostController) LikePost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.LikePost(ctx, postIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.LikePost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId}/likes [delete]
func (controller *PostController) UnlikePost(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.UnlikePost(ctx, postIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.UnlikePost")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Success      200   {object}  model.ServerCommentResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId}/comments [post]
func (controller *PostController) CreateComment(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")

	var payload model.ServerCommentCreateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		controller.Log.Debug("what happened?", zap.Error(err))
		modelErr := &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}
		return util.RecordAndSendValidationError(ctx, controller.Log, modelErr, "PostController.CreateComment")
	}

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.CreateComment(ctx, postIdParam, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.CreateComment")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId}/comments [get]
func (controller *PostController) GetComments(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")

	var validationErr *model.ValidationError

	response, err := controller.PostUsecase.GetComments(ctx, postIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.GetComments")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /posts/{postId}/comments/{commentId} [delete]
func (controller *PostController) DeleteComment(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	postIdParam := ctx.Params("postId")
	commentIdParam := ctx.Params("commentId")

	var validationErr *model.ValidationError

	err := controller.PostUsecase.DeleteComment(ctx, postIdParam, commentIdParam, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "PostController.DeleteComment")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

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

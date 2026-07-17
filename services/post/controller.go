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

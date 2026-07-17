package user

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/ferdian3456/virdanproject/services/post"
	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Repo       *Repository
	ServerRepo *server.Repository
	PostRepo   *post.Repository
	DB         *pgxpool.Pool
	Log        *zap.Logger
	Config     *koanf.Koanf
	Hub        *shared.WsHub
}

func NewService(repo *Repository, serverRepo *server.Repository, postRepo *post.Repository, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf, hub *shared.WsHub) *Service {
	return &Service{
		Repo:       repo,
		ServerRepo: serverRepo,
		PostRepo:   postRepo,
		DB:         db,
		Log:        log,
		Config:     config,
		Hub:        hub,
	}
}

func (service *Service) GetUserInfo(ctx fiber.Ctx, userId string) (UserResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetUserInfo")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response UserResponse

	v := shared.NewValidator()
	v.String("userId", userId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId))

	response, err = service.Repo.GetUserInfo(ctxContext, userId)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) DeleteAccount(ctx fiber.Ctx, userId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.DeleteAccount")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := shared.NewValidator()
	v.String("userId", userId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return err
	}

	var existsAndActive bool
	existsAndActive, err = service.Repo.CheckUserActive(ctxContext, userId)
	if err != nil {
		return err
	}
	if !existsAndActive {
		err = &shared.NotFoundError{
			Code:    shared.ERR_NOT_FOUND_CODE,
			Message: "User not found or already deleted",
			Param:   "",
		}
		return err
	}

	var ownedServers int
	ownedServers, err = service.ServerRepo.CountServersOwnedByUser(ctxContext, userId)
	if err != nil {
		return err
	}
	if ownedServers > 0 {
		err = &shared.ConflictError{
			Code:    shared.ERR_CONFLICT_CODE,
			Message: "You still own one or more servers. Transfer ownership or leave them before deleting your account.",
			Param:   "",
		}
		return err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	err = service.PostRepo.DeletePostsByAuthorId(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	err = service.PostRepo.DeleteCommentsByAuthorId(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	err = service.Repo.HardDeleteUser(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	err = service.Repo.RemoveAllAccessTokensFromCache(ctxContext, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to clear access token cache", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (service *Service) VerifyCurrentPassword(ctx fiber.Ctx, userId string, payload UserVerifyCurrentPasswordRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.VerifyCurrentPassword")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := shared.NewValidator()
	v.String("password", payload.Password).Required().MinLen(5).MaxLen(72)
	err = v.Validate()
	if err != nil {
		return err
	}

	var hash string
	hash, err = service.Repo.GetPasswordHashById(ctxContext, userId)
	if err != nil {
		return err
	}

	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(payload.Password)); bcryptErr != nil {
		err = &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: "Current password is incorrect",
			Param:   "password",
		}
		return err
	}
	return nil
}

func (service *Service) ChangePassword(ctx fiber.Ctx, userId string, payload UserChangePasswordRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ChangePassword")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := shared.NewValidator()
	v.String("currentPassword", payload.CurrentPassword).Required().MinLen(5).MaxLen(72)
	v.String("newPassword", payload.NewPassword).Required().MinLen(8).MaxLen(72)
	v.String("newPassword", payload.NewPassword).NotEqual(payload.CurrentPassword, "currentPassword")
	err = v.Validate()
	if err != nil {
		return err
	}

	var hash string
	hash, err = service.Repo.GetPasswordHashById(ctxContext, userId)
	if err != nil {
		return err
	}

	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(payload.CurrentPassword)); bcryptErr != nil {
		err = &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: "Current password is incorrect",
			Param:   "currentPassword",
		}
		return err
	}

	var newHash []byte
	newHash, err = bcrypt.GenerateFromPassword([]byte(payload.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to hash new password", zap.Error(err))
		return err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdatePasswordHash(ctxContext, userId, string(newHash), now)
	if err != nil {
		return err
	}
	return nil
}

const (
	emailChangeTTL      = 10 * time.Minute
	emailChangeCooldown = 60 * time.Second
	emailChangeMaxTries = 5
)

func (service *Service) RequestEmailChange(ctx fiber.Ctx, userId string, payload UserChangeEmailRequestRequest) (UserChangeEmailRequestResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.RequestEmailChange")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response UserChangeEmailRequestResponse

	v := shared.NewValidator()
	v.String("newEmail", payload.NewEmail).Required().Email().MaxLen(255)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	newEmail := strings.ToLower(strings.TrimSpace(payload.NewEmail))
	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("email.new", newEmail),
	)

	var ttl time.Duration
	ttl, err = service.Repo.GetEmailChangeSessionTTL(ctxContext, userId)
	if err != nil {
		return response, err
	}
	if ttl > emailChangeTTL-emailChangeCooldown {
		secondsLeft := int((ttl - (emailChangeTTL - emailChangeCooldown)).Seconds())
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Please wait %ds before requesting another code", secondsLeft),
			Param:   "newEmail",
		}
		return response, err
	}

	var currentEmail string
	currentEmail, err = service.Repo.GetUserEmail(ctxContext, userId)
	if err != nil {
		return response, err
	}
	if currentEmail == newEmail {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "New email must differ from current email",
			Param:   "newEmail",
		}
		return response, err
	}

	var emailTaken bool
	emailTaken, err = service.Repo.CheckEmailUnique(ctxContext, newEmail)
	if err != nil {
		return response, err
	}
	if emailTaken {
		err = &shared.ConflictError{
			Code:    shared.ERR_CONFLICT_CODE,
			Message: "Email is already registered",
			Param:   "newEmail",
		}
		return response, err
	}

	var otp string
	otp, err = shared.GenerateOTP()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate OTP", zap.Error(err))
		return response, err
	}
	otpHash := shared.HashSHA256(otp)
	expiresAt := time.Now().UTC().Add(emailChangeTTL).Unix()

	var tmpl *template.Template
	tmpl, err = template.ParseFS(shared.TemplateFS, "template/otp.html")
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to parse OTP template", zap.Error(err))
		return response, err
	}

	var bodyBuf bytes.Buffer
	err = tmpl.Execute(&bodyBuf, OTPTemplateData{OTP: otp, ExpiresIn: int64(emailChangeTTL.Minutes())})
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to execute OTP template", zap.Error(err))
		return response, err
	}

	emailCtx, emailSpan := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.RequestEmailChange.SendEmail")
	err = shared.SendEmail(
		service.Config.String("SMTP_HOST"),
		service.Config.Int("SMTP_PORT"),
		service.Config.String("SENDER_NAME"),
		service.Config.String("SENDER_EMAIL"),
		service.Config.String("SENDER_PASSWORD"),
		currentEmail,
		"Confirm your email change",
		bodyBuf.String(),
	)
	if err != nil {
		shared.RecordErrorTelemetry(emailCtx, emailSpan, err)
		emailSpan.End()
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to send OTP email", zap.Error(err))
		return response, err
	}
	emailSpan.End()

	err = service.Repo.SetEmailChangeSession(ctxContext, userId, newEmail, otpHash, emailChangeTTL)
	if err != nil {
		return response, err
	}

	response.OtpExpiresAt = expiresAt
	return response, nil
}

func (service *Service) ConfirmEmailChange(ctx fiber.Ctx, userId string, payload UserChangeEmailConfirmRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ConfirmEmailChange")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := shared.NewValidator()
	v.String("otp", payload.OTP).Required().Len(6)
	err = v.Validate()
	if err != nil {
		return err
	}

	var newEmail, otpHash string
	var attempts int
	newEmail, otpHash, attempts, err = service.Repo.GetEmailChangeSession(ctxContext, userId)
	if err != nil {
		return err
	}
	if newEmail == "" || otpHash == "" {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "No pending email change. Request a new code.",
			Param:   "otp",
		}
		return err
	}
	if attempts >= emailChangeMaxTries {
		_ = service.Repo.DeleteEmailChangeSession(ctxContext, userId)
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "Too many attempts. Request a new code.",
			Param:   "otp",
		}
		return err
	}

	if subtle.ConstantTimeCompare([]byte(otpHash), []byte(shared.HashSHA256(payload.OTP))) != 1 {
		_ = service.Repo.IncrementEmailChangeAttempts(ctxContext, userId)
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "Invalid code",
			Param:   "otp",
		}
		return err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateEmail(ctxContext, userId, newEmail, now)
	if err != nil {
		return err
	}

	_ = service.Repo.DeleteEmailChangeSession(ctxContext, userId)
	return nil
}

func (service *Service) UpdateNotificationPreferences(ctx fiber.Ctx, userId string, request UpdateNotificationPreferencesRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateNotificationPreferences")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	prefs := NotificationPrefs(request)

	now := time.Now()
	err = service.Repo.UpdateNotificationPrefs(ctxContext, userId, prefs, now)
	return err
}

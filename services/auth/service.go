package auth

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Repo   *Repository
	DB     *pgxpool.Pool
	Log    *zap.Logger
	Config *koanf.Koanf
	Hub    *shared.WsHub
}

func NewService(repo *Repository, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf, hub *shared.WsHub) *Service {
	return &Service{
		Repo:   repo,
		DB:     db,
		Log:    log,
		Config: config,
		Hub:    hub,
	}
}

func (service *Service) Login(ctx fiber.Ctx, payload UserLoginRequest) (shared.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.Login")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response shared.TokenResponse

	span.SetAttributes(attribute.String("user.email", payload.Email))

	v := shared.NewValidator()
	v.String("email", payload.Email).Required().Email().MaxLen(255)
	v.String("password", payload.Password).Required().MinLen(5).MaxLen(20)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	payload.Email = strings.ToLower(payload.Email)

	var userId, passwordHash string
	userId, passwordHash, err = service.Repo.GetUserAuthByEmail(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(payload.Password))
	if err != nil {
		err = &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: "Password is incorrect",
			Param:   "password",
		}
		return response, err
	}

	response, err = shared.GenerateTokenPair(userId, service.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate token pair", zap.Error(err))
		return shared.TokenResponse{}, err
	}

	now := time.Now().UTC()

	refreshToken := RefreshToken{
		Id:          uuid.New().String(),
		UserId:      userId,
		TokenHash:   shared.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(shared.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}
	err = service.Repo.CreateRefreshTokenNoTx(ctxContext, refreshToken)
	if err != nil {
		return response, err
	}

	err = service.Repo.SetAccessTokenInCache(ctxContext, response.AccessToken, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}

	service.Hub.CloseUser(userId)

	return response, nil
}

func (service *Service) StartSignup(ctx fiber.Ctx, payload UserSignupStartRequest) (UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.StartSignup")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response UserSignupStartResponse

	v := shared.NewValidator()
	v.String("email", payload.Email).Required().MinLen(5).MaxLen(255).Email()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	payload.Email = strings.ToLower(payload.Email)
	span.SetAttributes(attribute.String("user.email", payload.Email))

	var exists bool
	exists, err = service.Repo.CheckEmailUnique(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}
	if exists {
		err = &shared.ConflictError{
			Code:    shared.ERR_CONFLICT_CODE,
			Message: "Email is already registered",
			Param:   "email",
		}
		return response, err
	}

	var sessionExists bool
	var prevSessionId string
	sessionExists, prevSessionId, err = service.Repo.CheckSignupEmailSession(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}
	if sessionExists {
		err = service.Repo.DeleteSignupSession(ctxContext, prevSessionId)
		if err != nil {
			return response, err
		}
		err = service.Repo.DeleteEmailSignupSession(ctxContext, payload.Email)
		if err != nil {
			return response, err
		}
	}

	var otp string
	otp, err = shared.GenerateOTP()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate OTP", zap.Error(err))
		return response, err
	}

	otpHash := shared.HashSHA256(otp)
	sessionId := uuid.New().String()
	otpExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()

	response.SessionId = sessionId
	response.OtpExpiresAt = otpExpiresAt

	var tmpl *template.Template
	tmpl, err = template.ParseFS(shared.TemplateFS, "template/otp.html")
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to parse OTP template", zap.Error(err))
		return response, err
	}

	var bodyBuf bytes.Buffer
	err = tmpl.Execute(&bodyBuf, OTPTemplateData{OTP: otp, ExpiresIn: 5})
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to execute OTP template", zap.Error(err))
		return response, err
	}

	emailCtx, emailSpan := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.StartSignup.SendEmail")
	smtpHost := service.Config.String("SMTP_HOST")
	smtpPort := service.Config.Int("SMTP_PORT")
	senderName := service.Config.String("SENDER_NAME")
	senderEmail := service.Config.String("SENDER_EMAIL")
	senderPassword := service.Config.String("SENDER_PASSWORD")
	err = shared.SendEmail(smtpHost, smtpPort, senderName, senderEmail, senderPassword,
		payload.Email, "Register OTP Verification Code", bodyBuf.String())
	if err != nil {
		shared.RecordErrorTelemetry(emailCtx, emailSpan, err)
		emailSpan.End()
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to send OTP email", zap.Error(err))
		return response, err
	}
	emailSpan.End()

	err = service.Repo.SetSignupSession(ctxContext, sessionId, payload.Email, otpHash, otpExpiresAt)
	if err != nil {
		return response, err
	}

	err = service.Repo.SetSignupEmailSession(ctxContext, sessionId, payload.Email)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) VerifyOtp(ctx fiber.Ctx, payload UserVerifyOTPRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.VerifyOtp")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	v := shared.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	v.String("otp", payload.OTP).Required().Len(6)
	err = v.Validate()
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var otp OTPSignupData
	otp, err = service.Repo.GetOTPSignupSessionData(ctxContext, payload.SessionId)
	if err != nil {
		return err
	}

	if otp.OTP == "" {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "OTP does not exist or has expired",
			Param:   "otp",
		}
		return err
	}
	if time.Now().Unix() > otp.ExpiresAt {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "OTP has expired",
			Param:   "otp",
		}
		return err
	}
	if subtle.ConstantTimeCompare([]byte(otp.OTP), []byte(shared.HashSHA256(payload.OTP))) != 1 {
		err = &shared.BadRequestError{
			Code:    shared.ERR_BAD_REQUEST_CODE,
			Message: "OTP does not match",
			Param:   "otp",
		}
		return err
	}

	err = service.Repo.DeleteOTPState(ctxContext, payload.SessionId)
	if err != nil {
		return err
	}

	verifiedAt := time.Now().UTC().Unix()
	err = service.Repo.SetVerificationOTPState(ctxContext, payload.SessionId, verifiedAt)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) ResendOtp(ctx fiber.Ctx, payload UserResendOTPRequest) (UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ResendOtp")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response UserSignupStartResponse

	v := shared.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var data []interface{}
	data, err = service.Repo.GetOtpDataForResend(ctxContext, payload.SessionId)
	if err != nil {
		return response, err
	}

	if len(data) < 2 || data[0] == nil || data[1] == nil {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	emailStr, ok := data[0].(string)
	if !ok {
		err = fmt.Errorf("invalid email format from session data")
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}
	otpExpiresAtStr, ok := data[1].(string)
	if !ok {
		err = fmt.Errorf("invalid otp_expires_at format from session data")
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}

	var prevExpiresAt int64
	prevExpiresAt, err = strconv.ParseInt(otpExpiresAtStr, 10, 64)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to parse OTP expiresAt", zap.Error(err))
		return response, err
	}

	if time.Now().Unix() < prevExpiresAt {
		remainingSeconds := prevExpiresAt - time.Now().Unix()
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Please wait %s before requesting another OTP", shared.FormatRemainingTime(remainingSeconds)),
			Param:   "otp",
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
	newExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()
	response.SessionId = payload.SessionId
	response.OtpExpiresAt = newExpiresAt

	var tmpl *template.Template
	tmpl, err = template.ParseFS(shared.TemplateFS, "template/otp.html")
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to parse OTP template", zap.Error(err))
		return response, err
	}

	var bodyBuf bytes.Buffer
	err = tmpl.Execute(&bodyBuf, OTPTemplateData{OTP: otp, ExpiresIn: 5})
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to execute OTP template", zap.Error(err))
		return response, err
	}

	emailCtx, emailSpan := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ResendOtp.SendEmail")
	err = shared.SendEmail(
		service.Config.String("SMTP_HOST"),
		service.Config.Int("SMTP_PORT"),
		service.Config.String("SENDER_NAME"),
		service.Config.String("SENDER_EMAIL"),
		service.Config.String("SENDER_PASSWORD"),
		emailStr, "Register OTP Verification Code", bodyBuf.String(),
	)
	if err != nil {
		shared.RecordErrorTelemetry(emailCtx, emailSpan, err)
		emailSpan.End()
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to send OTP email", zap.Error(err))
		return response, err
	}
	emailSpan.End()

	err = service.Repo.UpdateSessionForResendOtp(ctxContext, payload.SessionId, otpHash, newExpiresAt)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) VerifyPassword(ctx fiber.Ctx, payload UserVerifyPasswordRequest) (shared.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.VerifyPassword")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response shared.TokenResponse

	v := shared.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	v.String("password", payload.Password).Required().MinLen(5).MaxLen(20)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var sessionData map[string]string
	sessionData, err = service.Repo.GetAllSessionData(ctxContext, payload.SessionId)
	if err != nil {
		return response, err
	}
	if len(sessionData) == 0 {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	if sessionData["step"] != SignupStepOTPVerified {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "Invalid signup step. Verify OTP first.",
			Param:   "sessionId",
		}
		return response, err
	}

	var emailTaken bool
	emailTaken, err = service.Repo.CheckEmailUnique(ctxContext, sessionData["email"])
	if err != nil {
		return response, err
	}
	if emailTaken {
		if delErr := service.Repo.DeleteSignupSession(ctxContext, payload.SessionId); delErr != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("Failed to delete signup session (non-fatal)", zap.Error(delErr))
		}
		if delErr := service.Repo.DeleteEmailSignupSession(ctxContext, sessionData["email"]); delErr != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("Failed to delete email signup session (non-fatal)", zap.Error(delErr))
		}
		err = &shared.ConflictError{
			Code:    shared.ERR_CONFLICT_CODE,
			Message: "Email has been registered since you started signup. Please restart.",
			Param:   "email",
		}
		return response, err
	}

	bcryptCtx, bcryptSpan := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.VerifyPassword.GenerateFromPassword")
	var hashedPassword []byte
	hashedPassword, err = bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		shared.RecordErrorTelemetry(bcryptCtx, bcryptSpan, err)
		bcryptSpan.End()
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to hash password", zap.Error(err))
		return response, err
	}
	bcryptSpan.End()

	userId := uuid.New().String()
	now := time.Now().UTC()

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	user := User{
		Id:        userId,
		Email:     sessionData["email"],
		Password:  string(hashedPassword),
		Settings:  []byte(shared.DEFAULT_USER_SETTINGS),
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	err = service.Repo.Register(ctxContext, tx, user)
	if err != nil {
		return response, err
	}

	response, err = shared.GenerateTokenPair(userId, service.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate token pair", zap.Error(err))
		return shared.TokenResponse{}, err
	}

	refreshToken := RefreshToken{
		Id:          uuid.New().String(),
		UserId:      userId,
		TokenHash:   shared.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(shared.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}
	err = service.Repo.CreateRefreshToken(ctxContext, tx, refreshToken)
	if err != nil {
		return response, err
	}

	err = service.Repo.SetAccessTokenInCache(ctxContext, response.AccessToken, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}
	err = service.Repo.DeleteSignupSession(ctxContext, payload.SessionId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to delete signup session", zap.Error(err))
		return response, err
	}
	err = service.Repo.DeleteEmailSignupSession(ctxContext, sessionData["email"])
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to delete email signup session", zap.Error(err))
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	return response, nil
}

func (service *Service) GetSignupStatus(ctx fiber.Ctx, sessionId string) (UserSignupStatus, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetSignupStatus")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response UserSignupStatus

	v := shared.NewValidator()
	v.String("sessionId", sessionId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionId))

	var data []interface{}
	data, err = service.Repo.GetSignupState(ctxContext, sessionId)
	if err != nil {
		return response, err
	}

	if len(data) == 0 || data[0] == nil {
		err = &shared.NotFoundError{
			Code:    shared.ERR_NOT_FOUND_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		err = fmt.Errorf("invalid signup step format from session data")
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}

	response.SessionId = sessionId
	response.Step = stepRaw

	return response, nil
}

func (service *Service) GetAccessToken(ctx fiber.Ctx, userId string, accessToken string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetAccessToken")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var hashedTokenFromCache string
	hashedTokenFromCache, err = service.Repo.GetAccessTokenInCache(ctxContext, userId)
	if err != nil {
		return err
	}

	hashedTokenFromClient := shared.HashToken(accessToken)
	if hashedTokenFromClient != hashedTokenFromCache {
		err = &shared.UnauthorizedError{
			Code:    shared.ERR_UNAUTHORIZED_CODE,
			Message: "Authorization token is expired",
			Param:   "accessToken",
		}
		return err
	}

	return nil
}

func (service *Service) Logout(ctx fiber.Ctx, userId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.Logout")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	now := time.Now().UTC()

	err = service.Repo.RevokeAllRefreshTokensByUserId(ctxContext, userId, now, now, userId)
	if err != nil {
		return err
	}

	err = service.Repo.RemoveAuthToken(ctxContext, userId)
	if err != nil {
		return err
	}

	service.Hub.CloseUser(userId)

	return nil
}

func (service *Service) RefreshToken(ctx fiber.Ctx, payload RefreshTokenRefreshRequest) (shared.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.RefreshToken")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response shared.TokenResponse

	v := shared.NewValidator()
	v.String("refreshToken", payload.RefreshToken).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	tokenHash := shared.HashToken(payload.RefreshToken)

	var refreshToken RefreshToken
	refreshToken, err = service.Repo.GetRefreshTokenByHash(ctxContext, tokenHash)
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", refreshToken.UserId),
		attribute.String("token.family", refreshToken.TokenFamily),
	)

	if refreshToken.RevokedAt != nil {
		now := time.Now().UTC()
		if revokeErr := service.Repo.RevokeAllRefreshTokensByUserId(ctxContext, refreshToken.UserId, now, now, refreshToken.UserId); revokeErr != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error(
				"Failed to revoke all refresh tokens during theft escalation",
				zap.String("user.id", refreshToken.UserId),
				zap.Error(revokeErr),
			)
		}

		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn(
			"Possible token theft detected — revoked refresh token reused",
			zap.String("user.id", refreshToken.UserId),
			zap.String("token.family", refreshToken.TokenFamily),
			zap.Time("revokedAt", *refreshToken.RevokedAt),
		)

		err = &shared.UnauthorizedError{
			Code:    shared.ERR_UNAUTHORIZED_CODE,
			Message: "Session expired. Please login again.",
			Param:   "refreshToken",
		}
		return response, err
	}

	if time.Now().UTC().After(refreshToken.ExpiresAt) {
		err = &shared.UnauthorizedError{
			Code:    shared.ERR_UNAUTHORIZED_CODE,
			Message: "Refresh token has expired",
			Param:   "refreshToken",
		}
		return response, err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	err = service.Repo.RevokeRefreshTokensByFamily(
		ctxContext, tx, refreshToken.UserId, refreshToken.TokenFamily, now, now, refreshToken.UserId)
	if err != nil {
		return response, err
	}

	response, err = shared.GenerateTokenPair(refreshToken.UserId, service.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate token pair", zap.Error(err))
		return shared.TokenResponse{}, err
	}

	newRefresh := RefreshToken{
		Id:          uuid.New().String(),
		UserId:      refreshToken.UserId,
		TokenHash:   shared.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(shared.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   refreshToken.UserId,
		UpdatedBy:   refreshToken.UserId,
	}
	err = service.Repo.CreateRefreshToken(ctxContext, tx, newRefresh)
	if err != nil {
		return response, err
	}

	err = service.Repo.SetAccessTokenInCache(ctxContext, response.AccessToken, refreshToken.UserId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	return response, nil
}

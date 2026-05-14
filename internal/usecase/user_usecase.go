package usecase

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type UserUsecase struct {
	UserRepository   *repository.UserRepository
	ServerRepository *repository.ServerRepository
	DB               *pgxpool.Pool
	Log              *zap.Logger
	Config           *koanf.Koanf
}

func NewUserUsecase(userRepository *repository.UserRepository, serverRepository *repository.ServerRepository, db *pgxpool.Pool, zap *zap.Logger, koanf *koanf.Koanf) *UserUsecase {
	return &UserUsecase{
		UserRepository:   userRepository,
		ServerRepository: serverRepository,
		DB:               db,
		Log:              zap,
		Config:           koanf,
	}
}

// Login authenticates a user and returns a fresh token pair.
func (usecase *UserUsecase) Login(ctx fiber.Ctx, payload model.UserLoginRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.Login")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.TokenResponse

	span.SetAttributes(attribute.String("user.username", payload.Username))

	v := util.NewValidator()
	v.String("username", payload.Username).Required().MinLen(4).MaxLen(22)
	v.String("password", payload.Password).Required().MinLen(5).MaxLen(20)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	payload.Username = strings.ToLower(payload.Username)

	var userId, passwordHash string
	userId, passwordHash, err = usecase.UserRepository.GetUserAuth(ctxContext, payload.Username)
	if err != nil {
		return response, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(payload.Password))
	if err != nil {
		err = &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: "Password is incorrect",
			Param:   "password",
		}
		return response, err
	}

	response, err = util.GenerateTokenPair(userId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate token pair", zap.Error(err))
		return model.TokenResponse{}, err
	}

	now := time.Now().UTC()

	refreshToken := model.RefreshToken{
		Id:          uuid.New().String(),
		UserId:      userId,
		TokenHash:   util.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(util.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}
	err = usecase.UserRepository.CreateRefreshTokenNoTx(ctxContext, refreshToken)
	if err != nil {
		return response, err
	}

	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, response.AccessToken, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}

	return response, nil
}

// StartSignup initiates a signup flow by sending an OTP to the provided email.
func (usecase *UserUsecase) StartSignup(ctx fiber.Ctx, payload model.UserSignupStartRequest) (model.UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.StartSignup")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.UserSignupStartResponse

	v := util.NewValidator()
	v.String("email", payload.Email).Required().MinLen(5).MaxLen(255).Email()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	payload.Email = strings.ToLower(payload.Email)
	span.SetAttributes(attribute.String("user.email", payload.Email))

	var exists bool
	exists, err = usecase.UserRepository.CheckEmailUnique(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}
	if exists {
		err = &model.ConflictError{
			Code:    constant.ERR_CONFLICT_CODE,
			Message: "Email is already registered",
			Param:   "email",
		}
		return response, err
	}

	var sessionExists bool
	var prevSessionId string
	sessionExists, prevSessionId, err = usecase.UserRepository.CheckSignupEmailSession(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}
	if sessionExists {
		err = usecase.UserRepository.DeleteSignupSession(ctxContext, prevSessionId)
		if err != nil {
			return response, err
		}
		err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, payload.Email)
		if err != nil {
			return response, err
		}
	}

	var otp string
	otp, err = util.GenerateOTP()
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate OTP", zap.Error(err))
		return response, err
	}

	otpHash := util.HashSHA256(otp)
	sessionId := uuid.New().String()
	otpExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()

	response.SessionId = sessionId
	response.OtpExpiresAt = otpExpiresAt

	var tmpl *template.Template
	tmpl, err = template.ParseFS(util.TemplateFS, "template/otp.html")
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to parse OTP template", zap.Error(err))
		return response, err
	}

	var bodyBuf bytes.Buffer
	err = tmpl.Execute(&bodyBuf, model.OTPTemplateData{OTP: otp, ExpiresIn: 5})
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to execute OTP template", zap.Error(err))
		return response, err
	}

	// Sub-span for SendEmail (slow op).
	emailCtx, emailSpan := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.StartSignup.SendEmail")
	smtpHost := usecase.Config.String("SMTP_HOST")
	smtpPort := usecase.Config.Int("SMTP_PORT")
	senderName := usecase.Config.String("SENDER_NAME")
	senderEmail := usecase.Config.String("SENDER_EMAIL")
	senderPassword := usecase.Config.String("SENDER_PASSWORD")
	err = util.SendEmail(smtpHost, smtpPort, senderName, senderEmail, senderPassword,
		payload.Email, "Register OTP Verification Code", bodyBuf.String())
	if err != nil {
		util.RecordErrorTelemetry(emailCtx, emailSpan, err)
		emailSpan.End()
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to send OTP email", zap.Error(err))
		return response, err
	}
	emailSpan.End()

	err = usecase.UserRepository.SetSignupSession(ctxContext, sessionId, payload.Email, otpHash, otpExpiresAt)
	if err != nil {
		return response, err
	}

	err = usecase.UserRepository.SetSignupEmailSession(ctxContext, sessionId, payload.Email)
	if err != nil {
		return response, err
	}

	return response, nil
}

// VerifyOtp validates the OTP for a signup session.
func (usecase *UserUsecase) VerifyOtp(ctx fiber.Ctx, payload model.UserVerifyOTPRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.VerifyOtp")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	v := util.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	v.String("otp", payload.OTP).Required().Len(6)
	err = v.Validate()
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var otp model.OTPSignupData
	otp, err = usecase.UserRepository.GetOTPSignupSessionData(ctxContext, payload.SessionId)
	if err != nil {
		return err
	}

	if otp.OTP == "" {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP does not exist or has expired",
			Param:   "otp",
		}
		return err
	}
	if time.Now().Unix() > otp.ExpiresAt {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP has expired",
			Param:   "otp",
		}
		return err
	}
	if subtle.ConstantTimeCompare([]byte(otp.OTP), []byte(util.HashSHA256(payload.OTP))) != 1 {
		err = &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: "OTP does not match",
			Param:   "otp",
		}
		return err
	}

	err = usecase.UserRepository.DeleteOTPState(ctxContext, payload.SessionId)
	if err != nil {
		return err
	}

	verifiedAt := time.Now().UTC().Unix()
	err = usecase.UserRepository.SetVerificationOTPState(ctxContext, payload.SessionId, verifiedAt)
	if err != nil {
		return err
	}

	return nil
}

// ResendOtp regenerates and resends an OTP, subject to cooldown.
func (usecase *UserUsecase) ResendOtp(ctx fiber.Ctx, payload model.UserResendOTPRequest) (model.UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ResendOtp")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.UserSignupStartResponse

	v := util.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var data []interface{}
	data, err = usecase.UserRepository.GetOtpDataForResend(ctxContext, payload.SessionId)
	if err != nil {
		return response, err
	}

	if len(data) < 2 || data[0] == nil || data[1] == nil {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	emailStr, ok := data[0].(string)
	if !ok {
		err = fmt.Errorf("invalid email format from session data")
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}
	otpExpiresAtStr, ok := data[1].(string)
	if !ok {
		err = fmt.Errorf("invalid otp_expires_at format from session data")
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}

	var prevExpiresAt int64
	prevExpiresAt, err = strconv.ParseInt(otpExpiresAtStr, 10, 64)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to parse OTP expiresAt", zap.Error(err))
		return response, err
	}

	if time.Now().Unix() < prevExpiresAt {
		remainingSeconds := prevExpiresAt - time.Now().Unix()
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Please wait %s before requesting another OTP", util.FormatRemainingTime(remainingSeconds)),
			Param:   "otp",
		}
		return response, err
	}

	var otp string
	otp, err = util.GenerateOTP()
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate OTP", zap.Error(err))
		return response, err
	}

	otpHash := util.HashSHA256(otp)
	newExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()
	response.SessionId = payload.SessionId
	response.OtpExpiresAt = newExpiresAt

	var tmpl *template.Template
	tmpl, err = template.ParseFS(util.TemplateFS, "template/otp.html")
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to parse OTP template", zap.Error(err))
		return response, err
	}

	var bodyBuf bytes.Buffer
	err = tmpl.Execute(&bodyBuf, model.OTPTemplateData{OTP: otp, ExpiresIn: 5})
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to execute OTP template", zap.Error(err))
		return response, err
	}

	emailCtx, emailSpan := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ResendOtp.SendEmail")
	err = util.SendEmail(
		usecase.Config.String("SMTP_HOST"),
		usecase.Config.Int("SMTP_PORT"),
		usecase.Config.String("SENDER_NAME"),
		usecase.Config.String("SENDER_EMAIL"),
		usecase.Config.String("SENDER_PASSWORD"),
		emailStr, "Register OTP Verification Code", bodyBuf.String(),
	)
	if err != nil {
		util.RecordErrorTelemetry(emailCtx, emailSpan, err)
		emailSpan.End()
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to send OTP email", zap.Error(err))
		return response, err
	}
	emailSpan.End()

	err = usecase.UserRepository.UpdateSessionForResendOtp(ctxContext, payload.SessionId, otpHash, newExpiresAt)
	if err != nil {
		return response, err
	}

	return response, nil
}

// VerifyUsername stores a unique username in the signup session.
func (usecase *UserUsecase) VerifyUsername(ctx fiber.Ctx, payload model.UserVerifyUsernameRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.VerifyUsername")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("signup.session_id", payload.SessionId),
		attribute.String("user.username", payload.Username),
	)

	v := util.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	v.String("username", payload.Username).Required().MinLen(4).MaxLen(22)
	err = v.Validate()
	if err != nil {
		return err
	}

	payload.Username = strings.ToLower(payload.Username)

	var data []interface{}
	data, err = usecase.UserRepository.GetSignupState(ctxContext, payload.SessionId)
	if err != nil {
		return err
	}

	if len(data) == 0 || data[0] == nil {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return err
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		err = fmt.Errorf("invalid signup step format from session data")
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Session data corrupt", zap.Error(err))
		return err
	}

	if stepRaw == model.SignupStepStart {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step. Verify OTP first.",
			Param:   "sessionId",
		}
		return err
	}

	var taken int
	taken, err = usecase.UserRepository.CheckUsernameUnique(ctxContext, payload.Username)
	if err != nil {
		return err
	}
	if taken > 0 {
		err = &model.ConflictError{
			Code:    constant.ERR_CONFLICT_CODE,
			Message: "Username is already taken",
			Param:   "username",
		}
		return err
	}

	err = usecase.UserRepository.SetVerificationUsernameState(ctxContext, payload.SessionId, payload.Username)
	if err != nil {
		return err
	}

	return nil
}

// VerifyPassword completes the signup flow by creating the user and issuing tokens.
func (usecase *UserUsecase) VerifyPassword(ctx fiber.Ctx, payload model.UserVerifyPasswordRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.VerifyPassword")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.TokenResponse

	v := util.NewValidator()
	v.String("sessionId", payload.SessionId).Required().UUID()
	v.String("password", payload.Password).Required().MinLen(5).MaxLen(20)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", payload.SessionId))

	var sessionData map[string]string
	sessionData, err = usecase.UserRepository.GetAllSessionData(ctxContext, payload.SessionId)
	if err != nil {
		return response, err
	}
	if len(sessionData) == 0 {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	if sessionData["step"] != model.SignupStepUsernameSet {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step. Set username first.",
			Param:   "sessionId",
		}
		return response, err
	}

	var usernameTaken, emailTaken bool
	usernameTaken, emailTaken, err = usecase.UserRepository.CheckUsernameOrEmailUnique(ctxContext, sessionData["username"], sessionData["email"])
	if err != nil {
		return response, err
	}
	if usernameTaken {
		err = &model.ConflictError{
			Code:    constant.ERR_CONFLICT_CODE,
			Message: "Username has been taken since you started signup. Please restart.",
			Param:   "username",
		}
		return response, err
	}
	if emailTaken {
		if delErr := usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId); delErr != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("Failed to delete signup session (non-fatal)", zap.Error(delErr))
		}
		if delErr := usecase.UserRepository.DeleteEmailSignupSession(ctxContext, sessionData["email"]); delErr != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("Failed to delete email signup session (non-fatal)", zap.Error(delErr))
		}
		err = &model.ConflictError{
			Code:    constant.ERR_CONFLICT_CODE,
			Message: "Email has been registered since you started signup. Please restart.",
			Param:   "email",
		}
		return response, err
	}

	// Sub-span: bcrypt.GenerateFromPassword (~100ms hot path).
	bcryptCtx, bcryptSpan := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.VerifyPassword.GenerateFromPassword")
	var hashedPassword []byte
	hashedPassword, err = bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		util.RecordErrorTelemetry(bcryptCtx, bcryptSpan, err)
		bcryptSpan.End()
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to hash password", zap.Error(err))
		return response, err
	}
	bcryptSpan.End()

	userId := uuid.New().String()
	now := time.Now().UTC()

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	user := model.User{
		Id:        userId,
		Username:  sessionData["username"],
		Email:     sessionData["email"],
		Password:  string(hashedPassword),
		Settings:  []byte("{}"),
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	err = usecase.UserRepository.Register(ctxContext, tx, user)
	if err != nil {
		return response, err
	}

	response, err = util.GenerateTokenPair(userId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate token pair", zap.Error(err))
		return model.TokenResponse{}, err
	}

	refreshToken := model.RefreshToken{
		Id:          uuid.New().String(),
		UserId:      userId,
		TokenHash:   util.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(util.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}
	err = usecase.UserRepository.CreateRefreshToken(ctxContext, tx, refreshToken)
	if err != nil {
		return response, err
	}

	// External ops BEFORE commit (Redis SET / DEL). On commit failure the orphan state is acceptable.
	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, response.AccessToken, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}
	err = usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to delete signup session", zap.Error(err))
		return response, err
	}
	err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, sessionData["email"])
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to delete email signup session", zap.Error(err))
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	return response, nil
}

// GetSignupStatus reports the current step of a signup session.
func (usecase *UserUsecase) GetSignupStatus(ctx fiber.Ctx, sessionId string) (model.UserSignupStatus, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetSignupStatus")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.UserSignupStatus

	v := util.NewValidator()
	v.String("sessionId", sessionId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionId))

	var data []interface{}
	data, err = usecase.UserRepository.GetSignupState(ctxContext, sessionId)
	if err != nil {
		return response, err
	}

	if len(data) == 0 || data[0] == nil {
		err = &model.NotFoundError{
			Code:    constant.ERR_NOT_FOUND_CODE,
			Message: "Signup session is expired or does not exist",
			Param:   "sessionId",
		}
		return response, err
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		err = fmt.Errorf("invalid signup step format from session data")
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Session data corrupt", zap.Error(err))
		return response, err
	}

	response.SessionId = sessionId
	response.Step = stepRaw

	return response, nil
}

// GetAccessToken verifies that the access token presented matches what is cached for the user.
func (usecase *UserUsecase) GetAccessToken(ctx fiber.Ctx, userId string, accessToken string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetAccessToken")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var hashedTokenFromCache string
	hashedTokenFromCache, err = usecase.UserRepository.GetAccessTokenInCache(ctxContext, userId)
	if err != nil {
		return err
	}

	hashedTokenFromClient := util.HashToken(accessToken)
	if hashedTokenFromClient != hashedTokenFromCache {
		err = &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_CODE,
			Message: "Authorization token is expired",
			Param:   "accessToken",
		}
		return err
	}

	return nil
}

// GetUserInfo returns the authenticated user's account info.
func (usecase *UserUsecase) GetUserInfo(ctx fiber.Ctx, userId string) (model.UserResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetUserInfo")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.UserResponse

	v := util.NewValidator()
	v.String("userId", userId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId))

	response, err = usecase.UserRepository.GetUserInfo(ctxContext, userId)
	if err != nil {
		return response, err
	}

	return response, nil
}

// Logout revokes all refresh tokens and clears the access-token cache.
func (usecase *UserUsecase) Logout(ctx fiber.Ctx, userId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.Logout")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	now := time.Now().UTC()

	err = usecase.UserRepository.RevokeAllRefreshTokensByUserId(ctxContext, userId, now, now, userId)
	if err != nil {
		return err
	}

	err = usecase.UserRepository.RemoveAuthToken(ctxContext, userId)
	if err != nil {
		return err
	}

	return nil
}

// RefreshToken rotates a refresh token, escalating to full revocation on detected theft.
func (usecase *UserUsecase) RefreshToken(ctx fiber.Ctx, payload model.RefreshTokenRefreshRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.RefreshToken")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.TokenResponse

	v := util.NewValidator()
	v.String("refreshToken", payload.RefreshToken).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	tokenHash := util.HashToken(payload.RefreshToken)

	var refreshToken model.RefreshToken
	refreshToken, err = usecase.UserRepository.GetRefreshTokenByHash(ctxContext, tokenHash)
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", refreshToken.UserId),
		attribute.String("token.family", refreshToken.TokenFamily),
	)

	// SECURITY: token reuse detected → escalate (revoke ALL user tokens).
	if refreshToken.RevokedAt != nil {
		now := time.Now().UTC()
		if revokeErr := usecase.UserRepository.RevokeAllRefreshTokensByUserId(ctxContext, refreshToken.UserId, now, now, refreshToken.UserId); revokeErr != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error(
				"Failed to revoke all refresh tokens during theft escalation",
				zap.String("user.id", refreshToken.UserId),
				zap.Error(revokeErr),
			)
		}

		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn(
			"Possible token theft detected — revoked refresh token reused",
			zap.String("user.id", refreshToken.UserId),
			zap.String("token.family", refreshToken.TokenFamily),
			zap.Time("revokedAt", *refreshToken.RevokedAt),
		)

		err = &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_CODE,
			Message: "Session expired. Please login again.",
			Param:   "refreshToken",
		}
		return response, err
	}

	if time.Now().UTC().After(refreshToken.ExpiresAt) {
		err = &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_CODE,
			Message: "Refresh token has expired",
			Param:   "refreshToken",
		}
		return response, err
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	err = usecase.UserRepository.RevokeRefreshTokensByFamily(
		ctxContext, tx, refreshToken.UserId, refreshToken.TokenFamily, now, now, refreshToken.UserId)
	if err != nil {
		return response, err
	}

	response, err = util.GenerateTokenPair(refreshToken.UserId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate token pair", zap.Error(err))
		return model.TokenResponse{}, err
	}

	newRefresh := model.RefreshToken{
		Id:          uuid.New().String(),
		UserId:      refreshToken.UserId,
		TokenHash:   util.HashToken(response.RefreshToken),
		TokenFamily: uuid.New().String(),
		ExpiresAt:   now.Add(util.RefreshTokenDuration),
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   refreshToken.UserId,
		UpdatedBy:   refreshToken.UserId,
	}
	err = usecase.UserRepository.CreateRefreshToken(ctxContext, tx, newRefresh)
	if err != nil {
		return response, err
	}

	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, response.AccessToken, refreshToken.UserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to cache access token", zap.Error(err))
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	return response, nil
}

// DeleteAccount soft-deletes the user, hard-deletes owned servers + memberships, and revokes all tokens.
//
// Flow:
//  1. Validate user exists and is not already deleted.
//  2. Hard delete owned servers (FK CASCADE handles roles/members/profiles/posts/comments/likes/invites).
//  3. Hard delete server_members rows for all other servers.
//  4. server_member_profiles retained (snapshot historical).
//  5. Posts/comments/likes in other servers retained (FK author_id; users row stays via soft delete).
//  6. Soft delete users row (set deleted_at).
//  7. Revoke all refresh tokens.
//  8. Invalidate access-token cache.
//
// MinIO objects intentionally stay orphan in Phase 1 (background cleanup job in Phase 2).
func (usecase *UserUsecase) DeleteAccount(ctx fiber.Ctx, userId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.DeleteAccount")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := util.NewValidator()
	v.String("userId", userId).Required().UUID()
	err = v.Validate()
	if err != nil {
		return err
	}

	var existsAndActive bool
	existsAndActive, err = usecase.UserRepository.CheckUserActive(ctxContext, userId)
	if err != nil {
		return err
	}
	if !existsAndActive {
		err = &model.NotFoundError{
			Code:    constant.ERR_NOT_FOUND_CODE,
			Message: "User not found or already deleted",
			Param:   "",
		}
		return err
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	err = usecase.ServerRepository.DeleteServersByOwnerId(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.DeleteAllServerMembersByUserId(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	err = usecase.UserRepository.SoftDeleteUser(ctxContext, tx, userId, now)
	if err != nil {
		return err
	}

	err = usecase.UserRepository.RevokeAllRefreshTokensByUserIdTx(ctxContext, tx, userId, now, now, userId)
	if err != nil {
		return err
	}

	err = usecase.UserRepository.RemoveAllAccessTokensFromCache(ctxContext, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to clear access token cache", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

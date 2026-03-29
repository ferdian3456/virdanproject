package usecase

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// func (usecase *UserUsecase) Register(ctx fiber.Ctx, payload model.UserCreateRequest) (model.TokenResponse, error) {
// 	ctxContext := ctx.Context()
// 	token := model.TokenResponse{}

// 	if payload.Username == "" {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Username is required to not be empty",
// 			Param:   "username",
// 		}
// 	} else if len(payload.Username) < 4 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Username must be at least 4 characters",
// 			Param:   "username",
// 		}
// 	} else if len(payload.Username) > 22 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "username must be at most 22 characters",
// 			Param:   "username",
// 		}
// 	}

// 	if payload.Email == "" {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Email is required to not be empty",
// 			Param:   "email",
// 		}
// 	} else if len(payload.Email) < 16 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "email must be at least 16 characters",
// 			Param:   "email",
// 		}
// 	} else if len(payload.Email) > 80 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Email must be at most 80 characters",
// 			Param:   "email",
// 		}
// 	}

// 	if payload.Password == "" {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Password is required to not be empty",
// 			Param:   "email",
// 		}
// 	} else if len(payload.Password) < 5 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Password must be at least 5 characters",
// 			Param:   "email",
// 		}
// 	} else if len(payload.Password) > 20 {
// 		return token, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Password must be at most 20 characters",
// 			Param:   "email",
// 		}
// 	}

// 	//err := usecase.UserRepository.CheckUsernameOrEmailUnique(ctxContext, payload.Username, payload.Email)
// 	//if err != nil {
// 	//	return token, err
// 	//}

// 	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
// 	if err != nil {
// 		return token, err
// 	}

// 	userUUID := uuid.New()
// 	now := time.Now().UTC()
// 	user := model.User{
// 		Id:             userUUID,
// 		Username:       payload.Username,
// 		Fullname:       strings.ToTitle(payload.Username),
// 		Bio:            nil,
// 		AvatarImageId:  nil,
// 		Email:          payload.Email,
// 		Password:       string(hashedPassword),
// 		Settings:       sonic.NoCopyRawMessage("{}"),
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userUUID,
// 		UpdateUserId:   userUUID,
// 	}

// 	serverUUID := uuid.New()
// 	server := model.Server{
// 		Id:            serverUUID,
// 		OwnerId:       userUUID,
// 		Name:          fmt.Sprintf("%s's server", strings.ToLower(payload.Username)),
// 		ShortName:     util.GenerateShortName(payload.Username),
// 		CategoryId:    nil,
// 		AvatarImageId: nil,
// 		BannerImageId: nil,
// 		Description:   nil,
// 		Settings: sonic.NoCopyRawMessage(`
// 		{
// 			"visibility": "private",
// 			"joinMode": "invite_only",
// 			"discoverable": false
// 		}`),
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userUUID,
// 		UpdateUserId:   userUUID,
// 	}

// 	serverRoleUUID := uuid.New()
// 	serverRole := model.ServerRole{
// 		Id:             serverRoleUUID,
// 		ServerId:       serverUUID,
// 		Name:           model.OwnerRole,
// 		Permissions:    sonic.NoCopyRawMessage(`{"*": true}`),
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userUUID,
// 		UpdateUserId:   userUUID,
// 	}

// 	serverMemberUUID := uuid.New()
// 	serverMember := model.ServerMember{
// 		Id:             serverMemberUUID,
// 		ServerId:       serverUUID,
// 		UserId:         userUUID,
// 		ServerRoleId:   serverRoleUUID,
// 		Status:         model.MemberStatusActive,
// 		JoinedAt:       now,
// 		LeftAt:         nil,
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userUUID,
// 		UpdateUserId:   userUUID,
// 	}

// 	serverMemberProfileUUID := uuid.New()
// 	serverMemberProfile := model.ServerMemberProfile{
// 		Id:             serverMemberProfileUUID,
// 		ServerMemberId: serverMemberUUID,
// 		ServerId:       serverUUID,
// 		UserId:         userUUID,
// 		Username:       user.Username,
// 		Fullname:       user.Fullname,
// 		Bio:            user.Bio,
// 		AvatarImageId:  nil,
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userUUID,
// 		UpdateUserId:   userUUID,
// 	}

// 	// start transaction
// 	tx, err := usecase.DB.Begin(ctx.Context())
// 	if err != nil {
// 		return token, err
// 	}

// 	defer tx.Rollback(ctxContext)

// 	err = usecase.UserRepository.Register(ctxContext, tx, user)
// 	if err != nil {
// 		return token, err
// 	}

// 	err = usecase.ServerRepository.CreateServer(ctxContext, tx, server)
// 	if err != nil {
// 		return token, err
// 	}

// 	err = usecase.ServerRepository.CreateRole(ctxContext, tx, serverRole)
// 	if err != nil {
// 		return token, err
// 	}

// 	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
// 	if err != nil {
// 		return token, err
// 	}

// 	err = usecase.ServerRepository.CreateServerMemberProfile(ctxContext, tx, serverMemberProfile)
// 	if err != nil {
// 		return token, err
// 	}

// 	err = tx.Commit(ctxContext)
// 	if err != nil {
// 		return token, err
// 	}

// 	token, err = util.GenerateTokenPair(user.Id, usecase.Config.String("JWT_SECRET_KEY"))
// 	if err != nil {
// 		return token, err
// 	}

// 	err = usecase.UserRepository.SetAuthTokenInCache(ctxContext, token.AccessToken, token.RefreshToken, user.Id)
// 	if err != nil {
// 		return token, err
// 	}

// 	return token, nil
// }

func (usecase *UserUsecase) Login(ctx fiber.Ctx, payload model.UserLoginRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	token := model.TokenResponse{}
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.Login")
	defer span.End()

	if payload.Username == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	} else if len(payload.Username) < 4 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	} else if len(payload.Username) > 22 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at most 22 characters",
			Param:   "username",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	}

	if payload.Password == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password is required to not be empty",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	} else if len(payload.Password) < 5 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at least 5 characters",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	} else if len(payload.Password) > 20 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at most 20 characters",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "Login")
		return token, err
	}

	payload.Username = strings.ToLower(payload.Username)

	userId, password, err := usecase.UserRepository.GetUserAuth(ctxContext, payload.Username)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(password), []byte(payload.Password))
	if err != nil {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password is incorrect",
			Param:   "password",
		}
	}

	token, err = util.GenerateTokenPair(userId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate token pair", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	now := time.Now().UTC()

	// Create refresh token in database
	refreshTokenHash := util.HashToken(token.RefreshToken)
	refreshTokenExpiresAt := now.Add(util.RefreshTokenDuration)
	tokenFamily := uuid.New().String()

	refreshTokenCreate := model.RefreshTokenCreate{
		Id:          uuid.New(),
		UserId:      userId,
		TokenHash:   refreshTokenHash,
		TokenFamily: tokenFamily,
		ExpiresAt:   refreshTokenExpiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
	}

	err = usecase.UserRepository.CreateRefreshTokenNoTx(ctxContext, refreshTokenCreate)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to create refresh token", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	// Store access token in Redis cache only
	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, token.AccessToken, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to set access token in cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	return token, nil
}

func (usecase *UserUsecase) GetUserInfo(ctx fiber.Ctx, userId uuid.UUID) (model.UserResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.GetUserInfo")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userId.String()))

	user, err := usecase.UserRepository.GetUserInfo(ctxContext, userId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return user, err
	}

	MINIO_URL := usecase.Config.String("MINIO_URL")
	MINIO_BUCKET_NAME := usecase.Config.String("MINIO_BUCKET_NAME")
	MINIO_HTTP := usecase.Config.String("MINIO_HTTP")

	if user.AvatarImage != nil {
		*user.AvatarImage = fmt.Sprintf("%s%s/%s/%s", MINIO_HTTP, MINIO_URL, MINIO_BUCKET_NAME, *user.AvatarImage)
	}

	return user, nil
}

func (usecase *UserUsecase) GetAccessToken(ctx fiber.Ctx, userId uuid.UUID, accessToken string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.GetAccessToken")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userId.String()))

	hashedTokenFromCache, err := usecase.UserRepository.GetAccessTokenInCache(ctxContext, userId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Hash the token from client before comparing with cached hash
	hashedTokenFromClient := util.HashToken(accessToken)

	if hashedTokenFromClient != hashedTokenFromCache {
		err := &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authorization token is expired",
			Param:   "accessToken",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	}

	return nil
}

func (usecase *UserUsecase) Logout(ctx fiber.Ctx, userId uuid.UUID) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.Logout")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userId.String()))

	now := time.Now().UTC()

	// Revoke all refresh tokens for this user
	err := usecase.UserRepository.RevokeAllRefreshTokensByUserId(ctxContext, userId, now, now, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to revoke refresh tokens", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Remove access token from Redis cache
	err = usecase.UserRepository.RemoveAuthToken(ctxContext, userId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateAvatar(ctx fiber.Ctx, userId uuid.UUID) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.UpdateAvatar")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userId.String()))

	fieldName := "avatar"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Avatar is required to not be empty",
			Param:   fieldName,
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateAvatar")
		return err
	}

	imageFile, imageSize, err := util.ValidateImage(fileHeader, fieldName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	avatarImageId := uuid.New()
	now := time.Now().UTC()
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	avatarImage := model.UserAvatarImage{
		Id:             avatarImageId,
		UserId:         userId,
		Bucket:         bucketName,
		ObjectKey:      fmt.Sprintf("user/avatar/%s.webp", avatarImageId),
		MimeType:       "webp",
		Size:           0,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		_ = tx.Rollback(ctxContext)
	}()

	fileName, err := usecase.UserRepository.GetUserAvatar(ctxContext, tx, userId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if fileName != "" {
		err = usecase.UserRepository.DeleteAvatarImage(ctxContext, tx, userId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}

		err = usecase.UserRepository.DeleteUserAvatar(ctxContext, bucketName, fileName)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	err = usecase.UserRepository.AddUserAvatar(ctxContext, tx, avatarImage)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = usecase.UserRepository.UploadUserAvatar(ctxContext, bucketName, avatarImage.ObjectKey, imageFile, imageSize)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) StartSignup(ctx fiber.Ctx, payload model.UserSignupStartRequest) (model.UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.StartSignup")
	defer span.End()

	response := model.UserSignupStartResponse{}

	if payload.Email == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is required to not be empty",
			Param:   "email",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "StartSignup")
		return response, err
	} else if len(payload.Email) < 16 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "email must be at least 16 characters",
			Param:   "email",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "StartSignup")
		return response, err
	} else if len(payload.Email) > 80 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email must be at most 80 characters",
			Param:   "email",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "StartSignup")
		return response, err
	}

	payload.Email = strings.ToLower(payload.Email)
	span.SetAttributes(attribute.String("user.email", payload.Email))

	exists1, err := usecase.UserRepository.CheckEmailUnique(ctxContext, payload.Email)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	if exists1 == 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is already exists",
			Param:   "email",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "StartSignup")
		return response, err
	}

	exists, emailSessionId, err := usecase.UserRepository.CheckSignupEmailSession(ctxContext, payload.Email)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	if exists {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Debug("email session exists, deleting previous session", zap.String("email", payload.Email))
		err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, emailSessionId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return response, err
		}
		err = usecase.UserRepository.DeleteSignupSession(ctxContext, emailSessionId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return response, err
		}
	}

	otp, err := util.GenerateOTP()
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate OTP", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	otpHash := util.HashSHA256(otp)
	sessionId := uuid.New()
	otpExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()

	response.SessionId = sessionId
	response.OtpExpiresAt = otpExpiresAt

	OtpTemplateData := model.OTPTemplateData{
		OTP:       otp,
		ExpiresIn: 5,
	}

	template, err := template.ParseFS(util.TemplateFS, "template/otp.html")
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to parse OTP template", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	var tmpl bytes.Buffer
	err = template.Execute(&tmpl, OtpTemplateData)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to execute OTP template", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	smtpHost := usecase.Config.String("SMTP_HOST")
	smtpPort := usecase.Config.Int("SMTP_PORT")
	senderName := usecase.Config.String("SENDER_NAME")
	senderEmail := usecase.Config.String("SENDER_EMAIL")
	senderPassword := usecase.Config.String("SENDER_PASSWORD")

	subject := "Register OTP Verification Code"
	err = util.SendEmail(smtpHost, smtpPort, senderName, senderEmail, senderPassword, payload.Email, subject, tmpl.String())
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to send OTP email", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	err = usecase.UserRepository.SetSignupSession(ctxContext, sessionId, payload.Email, otpHash, otpExpiresAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	err = usecase.UserRepository.SetSignupEmailSession(ctxContext, sessionId.String(), payload.Email)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	return response, nil
}

func (usecase *UserUsecase) VerifyOtp(ctx fiber.Ctx, payload model.UserVerifyOTPRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.VerifyOtp")
	defer span.End()

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionId.String()))

	if payload.OTP == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP is required to not be empty",
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	} else if len(payload.OTP) < 6 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP must be at least 6 characters",
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	}

	data, err := usecase.UserRepository.GetOTPSignupSessionData(ctxContext, sessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	otpRaw := data[0]
	expiresRaw := data[1]

	if otpRaw == nil || expiresRaw == nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP does not exists or expired",
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	}

	otpHash, ok := otpRaw.(string)
	if !ok {
		return fmt.Errorf("invalid OTP hash format")
	}

	otpExpiresAtStr, ok := expiresRaw.(string)
	if !ok {
		return fmt.Errorf("invalid OTP expiration format")
	}

	otpExpiresAt, err := strconv.ParseInt(otpExpiresAtStr, 10, 64)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if time.Now().Unix() > otpExpiresAt {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Otp is expired",
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	}

	if subtle.ConstantTimeCompare([]byte(otpHash), []byte(util.HashSHA256(payload.OTP))) != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Otp does not match",
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyOtp")
		return err
	}

	err = usecase.UserRepository.DeleteOTPState(ctxContext, sessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	verifiedAt := time.Now().UTC().Unix()
	err = usecase.UserRepository.SetVerificationOTPState(ctxContext, sessionId, verifiedAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) ResendOtp(ctx fiber.Ctx, payload model.UserResendOTPRequest) (model.UserSignupStartResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.ResendOtp")
	defer span.End()

	var response model.UserSignupStartResponse

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "ResendOtp")
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionId.String()))

	data, err := usecase.UserRepository.GetOtpDataForResend(ctxContext, sessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	emailRaw := data[0]
	expiresRaw := data[1]

	emailStr, ok := emailRaw.(string)
	if !ok {
		return response, fmt.Errorf("invalid email format")
	}

	otpExpiresAtStr, ok := expiresRaw.(string)
	if !ok {
		return response, fmt.Errorf("invalid OTP expiration format")
	}

	otpExpiresAt, err := strconv.ParseInt(otpExpiresAtStr, 10, 64)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	if time.Now().Unix() < otpExpiresAt {
		remainingSeconds := otpExpiresAt - time.Now().Unix()

		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Please wait %s before requesting another OTP", util.FormatRemainingTime(remainingSeconds)),
			Param:   "otp",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "ResendOtp")
		return response, err
	}

	otp, err := util.GenerateOTP()
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate OTP", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	otpHash := util.HashSHA256(otp)
	otpExpiresAt = time.Now().UTC().Add(5 * time.Minute).Unix()

	response.SessionId = sessionId
	response.OtpExpiresAt = otpExpiresAt

	OtpTemplateData := model.OTPTemplateData{
		OTP:       otp,
		ExpiresIn: 5,
	}

	template, err := template.ParseFS(util.TemplateFS, "template/otp.html")
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to parse OTP template", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	var tmpl bytes.Buffer
	err = template.Execute(&tmpl, OtpTemplateData)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to execute OTP template", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	smtpHost := usecase.Config.String("SMTP_HOST")
	smtpPort := usecase.Config.Int("SMTP_PORT")
	senderName := usecase.Config.String("SENDER_NAME")
	senderEmail := usecase.Config.String("SENDER_EMAIL")
	senderPassword := usecase.Config.String("SENDER_PASSWORD")

	subject := "Register OTP Verification Code"
	err = util.SendEmail(smtpHost, smtpPort, senderName, senderEmail, senderPassword, emailStr, subject, tmpl.String())
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to send OTP email", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	err = usecase.UserRepository.UpdateSessionForResendOtp(ctxContext, sessionId, otpHash, otpExpiresAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	return response, err
}

func (usecase *UserUsecase) VerifyUsername(ctx fiber.Ctx, payload model.UserVerifyUsernameRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.VerifyUsername")
	defer span.End()

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	}

	span.SetAttributes(
		attribute.String("signup.session_id", sessionId.String()),
		attribute.String("user.username", payload.Username),
	)

	if payload.Username == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	} else if len(payload.Username) < 4 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	} else if len(payload.Username) > 22 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "username must be at most 22 characters",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	}

	data, err := usecase.UserRepository.GetSignupState(ctxContext, sessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if data[0] == nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyUsername")
		return err
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		return fmt.Errorf("invalid signup step format")
	}

	if stepRaw == model.SignupStepStart {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step for this session",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyUsername")
		return err
	}

	exists, err := usecase.UserRepository.CheckUsernameUnique(ctxContext, payload.Username)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists == 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already taken",
			Param:   "username",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyUsername")
		return err
	}

	err = usecase.UserRepository.SetVerificationUsernameState(ctxContext, sessionId, payload.Username)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) VerifyPassword(ctx fiber.Ctx, payload model.UserVerifyPasswordRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.VerifyPassword")
	defer span.End()

	token := model.TokenResponse{}

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionId.String()))

	if payload.Password == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password is required to not be empty",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	} else if len(payload.Password) < 5 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at least 5 characters",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	} else if len(payload.Password) > 20 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at most 20 characters",
			Param:   "password",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	data, err := usecase.UserRepository.GetAllSessionData(ctxContext, sessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	if len(data) == 0 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	stepRaw, ok := data["step"]
	if !ok {
		return token, fmt.Errorf("step not found in session data")
	}

	if stepRaw != model.SignupStepUsernameSet {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step for this session",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	username, email, err := usecase.UserRepository.CheckUsernameOrEmailUnique(ctxContext, data["username"], data["email"])
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	if username == data["username"] {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already exist",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	if email == data["email"] {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Debug("email exists in verify password step, deleting session", zap.String("email", data["email"]))

		err = usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return token, err
		}

		err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, data["email"])
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return token, err
		}

		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is already exist",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "VerifyPassword")
		return token, err
	}

	err = usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, data["email"])
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate hashed password", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	userId := uuid.New()
	now := time.Now().UTC()
	user := model.User{
		Id:             userId,
		Username:       data["username"],
		Fullname:       strings.ToTitle(data["username"]),
		Bio:            nil,
		AvatarImageId:  nil,
		Email:          data["email"],
		Password:       string(hashedPassword),
		Settings:       sonic.NoCopyRawMessage("{}"),
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	err = usecase.UserRepository.RegisterNoTx(ctxContext, user)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	token, err = util.GenerateTokenPair(userId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate token pair", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	// Create refresh token in database
	refreshTokenHash := util.HashToken(token.RefreshToken)
	refreshTokenExpiresAt := now.Add(util.RefreshTokenDuration)
	tokenFamily := uuid.New().String()

	refreshTokenCreate := model.RefreshTokenCreate{
		Id:          uuid.New(),
		UserId:      userId,
		TokenHash:   refreshTokenHash,
		TokenFamily: tokenFamily,
		ExpiresAt:   refreshTokenExpiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
	}

	err = usecase.UserRepository.CreateRefreshTokenNoTx(ctxContext, refreshTokenCreate)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to create refresh token", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	// Store access token in Redis cache only
	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, token.AccessToken, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to set access token in cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	return token, nil
}

func (usecase *UserUsecase) GetSignupStatus(ctx fiber.Ctx, sessionId string) (model.UserSignupStatus, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.GetSignupStatus")
	defer span.End()

	response := model.UserSignupStatus{}
	sessionUUID, err := uuid.Parse(sessionId)
	if err != nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetSignupStatus")
		return response, err
	}

	span.SetAttributes(attribute.String("signup.session_id", sessionUUID.String()))

	data, err := usecase.UserRepository.GetSignupState(ctxContext, sessionUUID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	if data[0] == nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetSignupStatus")
		return response, err
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		return response, fmt.Errorf("invalid signup step format")
	}

	response.SessionId = sessionUUID
	response.Step = stepRaw

	return response, nil
}

func (usecase *UserUsecase) UpdateUsername(ctx fiber.Ctx, userId uuid.UUID, payload model.UsernameUpdateRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.UpdateUsername")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("user.username", payload.Username),
	)

	// Validate username
	if payload.Username == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	} else if len(payload.Username) < 4 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	} else if len(payload.Username) > 22 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "username must be at most 22 characters",
			Param:   "username",
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Message)
		return err
	}

	// Check if username is already taken
	exists, err := usecase.UserRepository.CheckUsernameUnique(ctxContext, payload.Username)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists == 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already taken",
			Param:   "username",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateUsername")
		return err
	}

	now := time.Now().UTC()

	err = usecase.UserRepository.UpdateUsername(ctxContext, userId, payload.Username, userId, now)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateFullname(ctx fiber.Ctx, userId uuid.UUID, payload model.FullnameUpdateRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.UpdateFullname")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("user.fullname", payload.Fullname),
	)

	// Validate fullname
	if payload.Fullname == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname is required to not be empty",
			Param:   "fullname",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateFullname")
		return err
	} else if len(payload.Fullname) < 4 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname must be at least 4 characters",
			Param:   "fullname",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateFullname")
		return err
	} else if len(payload.Fullname) > 40 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname must be at most 40 characters",
			Param:   "fullname",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateFullname")
		return err
	}

	now := time.Now().UTC()

	err := usecase.UserRepository.UpdateFullname(ctxContext, userId, payload.Fullname, userId, now)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateBio(ctx fiber.Ctx, userId uuid.UUID, payload model.BioUpdateRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.UpdateBio")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userId.String()))

	// No validation needed for bio
	// Convert empty string to nil for NULL in database
	var bioPtr *string
	if payload.Bio != "" {
		bioPtr = &payload.Bio
	}

	now := time.Now().UTC()

	err := usecase.UserRepository.UpdateBio(ctxContext, userId, bioPtr, userId, now)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (usecase *UserUsecase) RefreshToken(ctx fiber.Ctx, payload model.RefreshTokenRefreshRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	response := model.TokenResponse{}
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tracer.Start(ctxContext, "usecase.RefreshToken")
	defer span.End()

	// Validate refresh token
	if payload.RefreshToken == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Refresh token is required to not be empty",
			Param:   "refreshToken",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "RefreshToken")
		return response, err
	}

	// Hash the refresh token to find it in database
	tokenHash := util.HashToken(payload.RefreshToken)

	// Get refresh token from database
	refreshToken, err := usecase.UserRepository.GetRefreshTokenByHash(ctxContext, tokenHash)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to get refresh token by hash", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	// Check if token is revoked
	// If revoked, this could indicate token theft - security escalation
	if refreshToken.RevokedAt != nil {
		// SECURITY ESCALATION: Revoke ALL refresh tokens for this user
		// This kicks out any attacker who may have stolen a token
		now := time.Now().UTC()
		err := usecase.UserRepository.RevokeAllRefreshTokensByUserId(ctxContext, refreshToken.UserId, now, now, refreshToken.UserId)
		if err != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to revoke all tokens after revoked token detection", zap.Error(err))
			// Log error but continue - still return unauthorized to client
		}

		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("Possible token theft detected - attempt to use revoked refresh token",
			zap.String("userId", refreshToken.UserId.String()),
			zap.String("tokenFamily", refreshToken.TokenFamily),
			zap.Time("revokedAt", *refreshToken.RevokedAt),
		)

		err = &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Session expired. Please login again.",
			Param:   "refreshToken",
		}
		return response, err
	}

	// Check if token is expired
	if time.Now().UTC().After(refreshToken.ExpiresAt) {
		err := &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Refresh token is expired",
			Param:   "refreshToken",
		}
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("Attempt to use expired refresh token",
			zap.String("userId", refreshToken.UserId.String()),
			zap.String("tokenFamily", refreshToken.TokenFamily),
		)
		return response, err
	}

	// Start transaction for atomic operations
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}
	defer func() {
		_ = tx.Rollback(ctxContext)
	}()

	// Token rotation: revoke all tokens in the same family
	now := time.Now().UTC()
	err = usecase.UserRepository.RevokeRefreshTokensByFamily(ctxContext, tx, refreshToken.UserId, refreshToken.TokenFamily, now, now, refreshToken.UserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to revoke old refresh tokens", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	// Generate new token family for security (new family = new session lineage)
	newTokenFamily := uuid.New().String()

	// Generate new access token
	jwtSecretKey := usecase.Config.String("JWT_SECRET_KEY")
	accessToken, err := util.GenerateAccessToken(refreshToken.UserId, jwtSecretKey)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate access token", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	// Generate new refresh token
	newRefreshToken := uuid.New().String()
	newRefreshTokenHash := util.HashToken(newRefreshToken)
	newExpiresAt := now.Add(util.RefreshTokenDuration)

	// Create refresh token record
	refreshTokenCreate := model.RefreshTokenCreate{
		Id:          uuid.New(),
		UserId:      refreshToken.UserId,
		TokenHash:   newRefreshTokenHash,
		TokenFamily: newTokenFamily,
		ExpiresAt:   newExpiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   refreshToken.UserId,
	}

	err = usecase.UserRepository.CreateRefreshToken(ctxContext, tx, refreshTokenCreate)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to create new refresh token", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	// Set new tokens in cache
	err = usecase.UserRepository.SetAccessTokenInCache(ctxContext, accessToken, refreshToken.UserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to set new tokens in cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	// Commit transaction
	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	response = model.TokenResponse{
		AccessToken:           accessToken,
		AccessTokenExpiresIn:  int(util.AccessTokenDuration.Seconds()),
		RefreshToken:          newRefreshToken,
		RefreshTokenExpiresIn: int(util.RefreshTokenDuration.Seconds()),
		TokenType:             "Bearer",
	}

	return response, nil
}

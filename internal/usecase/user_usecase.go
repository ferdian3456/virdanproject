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

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
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

// func (usecase *UserUsecase) Register(ctx *fiber.Ctx, payload model.UserCreateRequest) (model.TokenResponse, error) {
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

func (usecase *UserUsecase) Login(ctx *fiber.Ctx, payload model.UserLoginRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	token := model.TokenResponse{}

	if payload.Username == "" {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
	} else if len(payload.Username) < 4 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
	} else if len(payload.Username) > 22 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "username must be at most 22 characters",
			Param:   "username",
		}
	}

	if payload.Password == "" {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password is required to not be empty",
			Param:   "password",
		}
	} else if len(payload.Password) < 5 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at least 5 characters",
			Param:   "password",
		}
	} else if len(payload.Password) > 20 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at most 20 characters",
			Param:   "password",
		}
	}

	payload.Username = strings.ToLower(payload.Username)

	userId, password, err := usecase.UserRepository.GetUserAuth(ctxContext, payload.Username)
	if err != nil {
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
		return token, err
	}

	err = usecase.UserRepository.SetAuthTokenInCache(ctxContext, token.AccessToken, token.RefreshToken, userId)
	if err != nil {
		return token, err
	}

	return token, nil
}

func (usecase *UserUsecase) GetUserInfo(ctx *fiber.Ctx, userId uuid.UUID) (model.UserResponse, error) {
	user, err := usecase.UserRepository.GetUserInfo(ctx.Context(), userId)
	if err != nil {
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

func (usecase *UserUsecase) GetAccessToken(ctx *fiber.Ctx, userId uuid.UUID, accessToken string) error {
	hashedTokenFromCache, err := usecase.UserRepository.GetAccessTokenInCache(ctx.Context(), userId)
	if err != nil {
		return err
	}

	// Hash the token from client before comparing with cached hash
	hashedTokenFromClient := util.HashToken(accessToken)

	if hashedTokenFromClient != hashedTokenFromCache {
		return &model.ValidationError{
			Code:    constant.ERR_NOT_FOUND_ERROR,
			Message: "Authorization token is expired",
			Param:   "accessToken",
		}
	}

	return nil
}

func (usecase *UserUsecase) Logout(ctx *fiber.Ctx, userId uuid.UUID) error {
	err := usecase.UserRepository.RemoveAuthToken(ctx.Context(), userId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateAvatar(ctx *fiber.Ctx, userId uuid.UUID) error {
	ctxContext := ctx.Context()

	fieldName := "avatar"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Avatar is required to not be empty",
			Param:   fieldName,
		}
	}

	imageFile, imageSize, err := util.ValidateImage(fileHeader, fieldName)
	if err != nil {
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
	tx, err := usecase.DB.Begin(ctx.Context())
	if err != nil {
		return err
	}

	defer tx.Rollback(ctxContext)

	fileName, err := usecase.UserRepository.GetUserAvatar(ctxContext, tx, userId)
	if err != nil {
		return err
	}

	if fileName != "" {
		err = usecase.UserRepository.DeleteAvatarImage(ctxContext, tx, userId)
		if err != nil {
			return err
		}

		err = usecase.UserRepository.DeleteUserAvatar(ctxContext, bucketName, fileName)
		if err != nil {
			return err
		}
	}

	err = usecase.UserRepository.AddUserAvatar(ctxContext, tx, avatarImage)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	err = usecase.UserRepository.UploadUserAvatar(ctxContext, bucketName, avatarImage.ObjectKey, imageFile, imageSize)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) StartSignup(ctx *fiber.Ctx, payload model.UserSignupStartRequest) (model.UserSignupStartResponse, error) {
	ctxContext := ctx.Context()

	response := model.UserSignupStartResponse{}

	if payload.Email == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is required to not be empty",
			Param:   "email",
		}
	} else if len(payload.Email) < 16 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "email must be at least 16 characters",
			Param:   "email",
		}
	} else if len(payload.Email) > 80 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email must be at most 80 characters",
			Param:   "email",
		}
	}

	payload.Email = strings.ToLower(payload.Email)

	exists1, err := usecase.UserRepository.CheckEmailUnique(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}

	if exists1 == 1 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is already exists",
			Param:   "email",
		}
	}

	exists, emailSessionId, err := usecase.UserRepository.CheckSignupEmailSession(ctxContext, payload.Email)
	if err != nil {
		return response, err
	}

	if exists {
		usecase.Log.Debug("email session is exists, preparing to delete email and signup session", zap.String("email", payload.Email))
		err = usecase.UserRepository.DeleteEmailSignupSession(ctx.Context(), emailSessionId)
		if err != nil {
			return response, err
		}
		err = usecase.UserRepository.DeleteSignupSession(ctx.Context(), emailSessionId)
		if err != nil {
			return response, err
		}
	}

	otp, err := util.GenerateOTP()
	if err != nil {
		return response, err
	}

	otpHash := util.HashSHA256(otp)

	sessionId := uuid.New()

	// save session
	otpExpiresAt := time.Now().UTC().Add(5 * time.Minute).Unix()

	response.SessionId = sessionId
	response.OtpExpiresAt = otpExpiresAt

	OtpTemplateData := model.OTPTemplateData{
		OTP:       otp,
		ExpiresIn: 5,
	}

	template, err := template.ParseFS(util.TemplateFS, "template/otp.html")
	if err != nil {
		return response, err
	}

	var tmpl bytes.Buffer
	err = template.Execute(&tmpl, OtpTemplateData)
	if err != nil {
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
		return response, err
	}

	err = usecase.UserRepository.SetSignupSession(ctx.Context(), sessionId, payload.Email, otpHash, otpExpiresAt)
	if err != nil {
		return response, err
	}

	err = usecase.UserRepository.SetSignupEmailSession(ctx.Context(), sessionId.String(), payload.Email)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *UserUsecase) VerifyOtp(ctx *fiber.Ctx, payload model.UserVerifyOTPRequest) error {
	ctxContext := ctx.Context()

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
	}

	if payload.OTP == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP is required to not be empty",
			Param:   "otp",
		}
	} else if len(payload.OTP) < 6 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP must be at least 6 characters",
			Param:   "otp",
		}
	}

	data, err := usecase.UserRepository.GetOTPSignupSessionData(ctx.Context(), sessionId)
	if err != nil {
		return err
	}

	otpRaw := data[0]
	expiresRaw := data[1]

	if otpRaw == nil || expiresRaw == nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "OTP does not exists or expired",
			Param:   "otp",
		}
	}

	otpHash, ok := otpRaw.(string)
	if !ok {
		return err
	}

	otpExpiresAtStr, ok := expiresRaw.(string)
	if !ok {
		return err
	}

	otpExpiresAt, err := strconv.ParseInt(otpExpiresAtStr, 10, 64)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(otpHash), []byte(util.HashSHA256(payload.OTP))) != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Otp does not match",
			Param:   "otp",
		}
	}

	if time.Now().Unix() > otpExpiresAt {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Otp is expired",
			Param:   "otp",
		}
	}

	err = usecase.UserRepository.DeleteOTPState(ctxContext, sessionId)
	if err != nil {
		return err
	}

	verifiedAt := time.Now().UTC().Unix()
	err = usecase.UserRepository.SetVerificationOTPState(ctxContext, sessionId, verifiedAt)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) VerifyUsername(ctx *fiber.Ctx, payload model.UserVerifyUsernameRequest) error {
	ctxContext := ctx.Context()

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
	}

	if payload.Username == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
	} else if len(payload.Username) < 4 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
	} else if len(payload.Username) > 22 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "username must be at most 22 characters",
			Param:   "username",
		}
	}

	data, err := usecase.UserRepository.GetSignupState(ctx.Context(), sessionId)
	if err != nil {
		return err
	}

	if data[0] == nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		return err
	}

	if stepRaw == model.SignupStepStart {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step for this session",
			Param:   "sessionId",
		}
	}

	exists, err := usecase.UserRepository.CheckUsernameUnique(ctxContext, payload.Username)
	if err != nil {
		return err
	}

	if exists == 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already taken",
			Param:   "username",
		}
	}

	err = usecase.UserRepository.SetVerificationUsernameState(ctxContext, sessionId, payload.Username)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) VerifyPassword(ctx *fiber.Ctx, payload model.UserVerifyPasswordRequest) (model.TokenResponse, error) {
	ctxContext := ctx.Context()
	token := model.TokenResponse{}

	sessionId, err := uuid.Parse(payload.SessionId)
	if err != nil {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
	}

	if payload.Password == "" {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password is required to not be empty",
			Param:   "password",
		}
	} else if len(payload.Password) < 5 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at least 5 characters",
			Param:   "password",
		}
	} else if len(payload.Password) > 20 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Password must be at most 20 characters",
			Param:   "password",
		}
	}

	data, err := usecase.UserRepository.GetAllSessionData(ctx.Context(), sessionId)
	if err != nil {
		return token, err
	}

	if len(data) == 0 {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
	}

	stepRaw, ok := data["step"]
	if !ok {
		return token, err
	}

	if stepRaw != model.SignupStepUsernameSet {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid signup step for this session",
			Param:   "sessionId",
		}
	}

	username, email, err := usecase.UserRepository.CheckUsernameOrEmailUnique(ctxContext, data["username"], data["email"])
	if err != nil {
		return token, err
	}

	if username == data["username"] {
		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already exist",
			Param:   "sessionId",
		}
	}

	if email == data["email"] {
		usecase.Log.Debug("email is exists in verify password step, preparing to delete email and signup session",
			zap.String("email", data["email"]))

		err = usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId)
		if err != nil {
			return token, err
		}

		err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, data["email"])
		if err != nil {
			return token, err
		}

		return token, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Email is already exist",
			Param:   "sessionId",
		}
	}

	err = usecase.UserRepository.DeleteSignupSession(ctxContext, payload.SessionId)
	if err != nil {
		return token, err
	}

	err = usecase.UserRepository.DeleteEmailSignupSession(ctxContext, data["email"])
	if err != nil {
		return token, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
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

	err = usecase.UserRepository.RegisterNoTx(ctx.Context(), user)
	if err != nil {
		return token, err
	}

	token, err = util.GenerateTokenPair(userId, usecase.Config.String("JWT_SECRET_KEY"))
	if err != nil {
		return token, err
	}

	err = usecase.UserRepository.SetAuthTokenInCache(ctxContext, token.AccessToken, token.RefreshToken, userId)
	if err != nil {
		return token, err
	}

	return token, nil
}

func (usecase *UserUsecase) GetSignupStatus(ctx *fiber.Ctx, sessionId string) (model.UserSignupStatus, error) {
	response := model.UserSignupStatus{}
	sessionUUID, err := uuid.Parse(sessionId)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid session id",
			Param:   "sessionId",
		}
	}

	data, err := usecase.UserRepository.GetSignupState(ctx.Context(), sessionUUID)
	if err != nil {
		return response, err
	}

	if data[0] == nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Signup session is expired or not exists",
			Param:   "sessionId",
		}
	}

	stepRaw, ok := data[0].(string)
	if !ok {
		return response, err
	}

	response.SessionId = sessionUUID
	response.Step = stepRaw

	return response, nil
}

func (usecase *UserUsecase) UpdateUsername(ctx *fiber.Ctx, userId uuid.UUID, payload model.UsernameUpdateRequest) error {
	ctxContext := ctx.Context()

	// Validate username
	if payload.Username == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is required to not be empty",
			Param:   "username",
		}
	} else if len(payload.Username) < 4 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username must be at least 4 characters",
			Param:   "username",
		}
	} else if len(payload.Username) > 22 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "username must be at most 22 characters",
			Param:   "username",
		}
	}

	// Check if username is already taken
	exists, err := usecase.UserRepository.CheckUsernameUnique(ctxContext, payload.Username)
	if err != nil {
		return err
	}

	if exists == 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Username is already taken",
			Param:   "username",
		}
	}

	now := time.Now().UTC()

	err = usecase.UserRepository.UpdateUsername(ctxContext, userId, payload.Username, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateFullname(ctx *fiber.Ctx, userId uuid.UUID, payload model.FullnameUpdateRequest) error {
	ctxContext := ctx.Context()

	// Validate fullname
	if payload.Fullname == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname is required to not be empty",
			Param:   "fullname",
		}
	} else if len(payload.Fullname) < 4 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname must be at least 4 characters",
			Param:   "fullname",
		}
	} else if len(payload.Fullname) > 40 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Fullname must be at most 40 characters",
			Param:   "fullname",
		}
	}

	now := time.Now().UTC()

	err := usecase.UserRepository.UpdateFullname(ctxContext, userId, payload.Fullname, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *UserUsecase) UpdateBio(ctx *fiber.Ctx, userId uuid.UUID, payload model.BioUpdateRequest) error {
	ctxContext := ctx.Context()

	// No validation needed for bio
	// Convert empty string to nil for NULL in database
	var bioPtr *string
	if payload.Bio != "" {
		bioPtr = &payload.Bio
	}

	now := time.Now().UTC()

	err := usecase.UserRepository.UpdateBio(ctxContext, userId, bioPtr, userId, now)
	if err != nil {
		return err
	}

	return nil
}

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type UserRepository struct {
	Log      *zap.Logger
	Config   *koanf.Koanf
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewUserRepository(zap *zap.Logger, koanf *koanf.Koanf, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *UserRepository {
	return &UserRepository{
		Log:      zap,
		Config:   koanf,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

// Postgresql
func (repository *UserRepository) Register(ctx context.Context, tx pgx.Tx, user model.User) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.Register")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "INSERT"),
		attribute.String("user.username", user.Username),
	)

	query := "INSERT INTO users (id,username,fullname,bio,avatar_image_id, email,password, settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"

	_, err := tx.Exec(ctx, query, user.Id, user.Username, user.Fullname, user.Bio, user.AvatarImageId, user.Email, user.Password, user.Settings, user.CreateDatetime, user.UpdateDatetime, user.CreateUserId, user.UpdateUserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to register user", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// Postgresql
func (repository *UserRepository) RegisterNoTx(ctx context.Context, user model.User) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RegisterNoTx")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "INSERT"),
		attribute.String("user.username", user.Username),
	)

	query := "INSERT INTO users (id,username,fullname,bio,avatar_image_id, email, password, settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"

	_, err := repository.DB.Exec(ctx, query, user.Id, user.Username, user.Fullname, user.Bio, user.AvatarImageId, user.Email, user.Password, user.Settings, user.CreateDatetime, user.UpdateDatetime, user.CreateUserId, user.UpdateUserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to register user without transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) CheckUsernameOrEmailUnique(ctx context.Context, username string, email string) (string, string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.CheckUsernameOrEmailUnique")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.username", username),
		attribute.String("user.email", email),
	)

	query := "SELECT username,email FROM users WHERE username=$1 OR email=$2 LIMIT 1"

	var existUsername string
	var existEmail string
	err := repository.DB.QueryRow(ctx, query, username, email).Scan(&existUsername, &existEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return existUsername, existEmail, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check username or email uniqueness", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return existUsername, existEmail, err
	}

	return existUsername, existEmail, nil
}

func (repository *UserRepository) GetUserAuth(ctx context.Context, username string) (uuid.UUID, string, error) {
	// 1. Start Tracing Span
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserAuth")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	query := "SELECT id,password FROM users WHERE username=$1 LIMIT 1"

	// 2. Add DB Attributes
	dbSystem := repository.Config.String("DB_SYSTEM")

	span.SetAttributes(
		attribute.String("db.system", dbSystem),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.user.username", username),
	)

	var id uuid.UUID
	var passwordHash string

	err = repository.DB.QueryRow(ctx, query, username).Scan(&id, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return id, passwordHash, &model.BadRequestError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Username is not found",
				Param:   "username",
			}
		}
		// 3. Log with Trace Correlation and Record Error
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user auth by username", zap.Error(err))
		return id, passwordHash, err
	}

	return id, passwordHash, nil
}

func (repository *UserRepository) GetUserInfo(ctx context.Context, id uuid.UUID) (model.UserResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetUserInfo")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", id.String()),
	)

	query := `SELECT A.id,A.username,A.fullname,A.email,B.object_key,A.bio,A.create_datetime,A.update_datetime
			FROM users A
			LEFT JOIN user_avatar_images B ON A.id = B.user_id
			WHERE A.id=$1
			LIMIT 1`

	user := model.UserResponse{}
	err := repository.DB.QueryRow(ctx, query, id).Scan(&user.Id, &user.Username, &user.Fullname, &user.Email, &user.AvatarImage, &user.Bio, &user.CreateDatetime, &user.UpdateDatetime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_ERROR,
				Message: "User not found",
				Param:   "userId",
			}
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user info by ID", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return user, err
	}

	return user, nil
}

// Redis - Cache
func (repository *UserRepository) SetAccessTokenInCache(ctx context.Context, accessToken string, userId uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "SET"),
		attribute.String("user.id", userId.String()),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	// Hash token before storing in Redis for security
	hashedAccessToken := util.HashToken(accessToken)

	err = repository.DBCache.Set(ctx, accessTokenKey, hashedAccessToken, 15*time.Minute).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set access token in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetAccessTokenInCache(ctx context.Context, userId uuid.UUID) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "GET"),
		attribute.String("user.id", userId.String()),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)
	hashedToken, err := repository.DBCache.Get(ctx, accessTokenKey).Result()
	if err == redis.Nil {
		return hashedToken, &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authorization token not found or expired",
			Param:   "accessToken",
		}
	} else if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get access token from cache", zap.Error(err))
		return hashedToken, err
	}

	return hashedToken, nil
}

func (repository *UserRepository) RemoveAuthToken(ctx context.Context, userId uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RemoveAuthToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId.String()),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	err := repository.DBCache.Del(ctx, accessTokenKey).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to remove access token from cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// RevokeAllRefreshTokensByUserId revokes all active refresh tokens for a user
func (repository *UserRepository) RevokeAllRefreshTokensByUserId(ctx context.Context, userId uuid.UUID, revokedAt time.Time, updatedAt time.Time, updatedBy uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RevokeAllRefreshTokensByUserId")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId.String()),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND revoked_at IS NULL`

	_, err := repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke all refresh tokens for user", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) UploadUserAvatar(ctx context.Context, bucketName string, imageName string, imageFile *bytes.Reader, imageSize int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.UploadUserAvatar")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "minio"),
		attribute.String("db.operation", "PUT"),
		attribute.String("db.minio.bucket", bucketName),
		attribute.String("db.minio.object", imageName),
	)

	_, err := repository.DBObject.PutObject(ctx, bucketName, imageName, imageFile, imageSize,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upload user avatar to object storage", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) GetUserAvatar(ctx context.Context, tx pgx.Tx, userId uuid.UUID) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetUserAvatar")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId.String()),
	)

	query := "SELECT object_key FROM user_avatar_images WHERE user_id=$1 LIMIT 1"

	var objectKey string
	err := tx.QueryRow(ctx, query, userId).Scan(&objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user avatar", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return objectKey, err
	}

	return objectKey, nil
}

func (repository *UserRepository) DeleteUserAvatar(ctx context.Context, bucketName string, fileName string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.DeleteUserAvatar")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "minio"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.minio.bucket", bucketName),
		attribute.String("db.minio.object", fileName),
	)

	err := repository.DBObject.RemoveObject(ctx, bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete user avatar from object storage", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) DeleteAvatarImage(ctx context.Context, tx pgx.Tx, userId uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.DeleteAvatarImage")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "DELETE"),
		attribute.String("user.id", userId.String()),
	)

	query := "DELETE FROM user_avatar_images WHERE user_id=$1"

	_, err := tx.Exec(ctx, query, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete avatar image from database", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) AddUserAvatar(ctx context.Context, tx pgx.Tx, avatar model.UserAvatarImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.AddUserAvatar")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "INSERT"),
		attribute.String("user.id", avatar.UserId.String()),
		attribute.String("db.minio.bucket", avatar.Bucket),
		attribute.String("db.minio.object", avatar.ObjectKey),
	)

	query := "INSERT INTO user_avatar_images (id, user_id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"

	_, err := tx.Exec(ctx, query, avatar.Id, avatar.UserId, avatar.Bucket, avatar.ObjectKey, avatar.MimeType, avatar.Size, avatar.CreateDatetime, avatar.UpdateDatetime, avatar.CreateUserId, avatar.UpdateUserId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to add user avatar to database", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) SetSignupSession(ctx context.Context, sessionId uuid.UUID, email string, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetSignupSession")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err = repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"email":          email,
		"otp":            otp,
		"otp_expires_at": otpExpiresAt,
		"step":           model.SignupStepStart,
		"created_at":     time.Now().Unix(),
	}).Err()

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set signup session in cache", zap.Error(err))
		return err
	}

	err = repository.DBCache.Expire(ctx, key, 30*time.Minute).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set expiration for signup session", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) SetSignupEmailSession(ctx context.Context, sessionId string, email string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetSignupEmailSession")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "SET"),
		attribute.String("user.email", email),
	)

	key := fmt.Sprintf("signup_email:%s", email)

	err = repository.DBCache.Set(ctx, key, sessionId, 30*time.Minute).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set signup email session in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetOTPSignupSessionData(ctx context.Context, sessionId uuid.UUID) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetOTPSignupSessionData")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "otp", "otp_expires_at").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP signup session data", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) GetOtpDataForResend(ctx context.Context, sessionId uuid.UUID) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetOtpDataForResend")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "email", "otp_expires_at").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP session data for resend", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) GetSignupState(ctx context.Context, sessionId uuid.UUID) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetSignupState")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "step").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get signup state from cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) DeleteOTPState(ctx context.Context, sessionId uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.DeleteOTPState")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HDEL"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HDel(ctx, key, "otp", "otp_expires_at").Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete OTP state from cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateSessionForResendOtp(ctx context.Context, sessionId uuid.UUID, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.UpdateSessionForResendOtp")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"otp":            otp,
		"otp_expires_at": otpExpiresAt,
	}).Err()

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update session for resend otp from cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) SetVerificationOTPState(ctx context.Context, sessionId uuid.UUID, verifiedAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.SetVerificationOTPState")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"step":            model.SignupStepOTPVerified,
		"otp_verified_at": verifiedAt,
	}).Err()

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set verification OTP state in cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) CheckUsernameUnique(ctx context.Context, username string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.CheckUsernameUnique")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.username", username),
	)

	query := "SELECT 1 FROM users WHERE username=$1 LIMIT 1"

	var exists int
	err := repository.DB.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check username uniqueness", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return exists, err
	}

	return exists, nil
}

func (repository *UserRepository) CheckEmailUnique(ctx context.Context, email string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckEmailUnique")

	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.email", email),
	)

	query := "SELECT 1 FROM users WHERE email=$1 LIMIT 1"

	var exists int
	err = repository.DB.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check email uniqueness", zap.Error(err))
		return false, err
	}

	return true, nil
}

func (repository *UserRepository) SetVerificationUsernameState(ctx context.Context, sessionId uuid.UUID, username string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.SetVerificationUsernameState")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId.String()),
		attribute.String("user.username", username),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"step":     model.SignupStepUsernameSet,
		"username": username,
	}).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set verification username state in cache", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) GetAllSessionData(ctx context.Context, sessionId uuid.UUID) (map[string]string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetAllSessionData")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HGETALL"),
		attribute.String("signup.session_id", sessionId.String()),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HGetAll(ctx, key).Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get all signup session data", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return vals, nil
}

func (repository *UserRepository) CheckSignupEmailSession(ctx context.Context, email string) (bool, string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckSignupEmailSession")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "GET"),
		attribute.String("user.email", email),
	)

	key := fmt.Sprintf("signup_email:%s", email)
	sessionId, err := repository.DBCache.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, sessionId, nil
	} else if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check signup email session", zap.Error(err))
		return false, sessionId, err
	}

	return true, sessionId, nil
}

func (repository *UserRepository) DeleteSignupSession(ctx context.Context, sessionId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteSignupSession")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err = repository.DBCache.Del(ctx, key).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete signup session", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) DeleteEmailSignupSession(ctx context.Context, sesisonId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteEmailSignupSession")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
	)

	key := fmt.Sprintf("signup_email:%s", sesisonId)

	err = repository.DBCache.Del(ctx, key).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete email signup session", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateUsername(ctx context.Context, userId uuid.UUID, username string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.UpdateUsername")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId.String()),
		attribute.String("user.username", username),
	)

	query := "UPDATE users SET username = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, username, updateDatetime, updateUserId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update username", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateFullname(ctx context.Context, userId uuid.UUID, fullname string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.UpdateFullname")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId.String()),
		attribute.String("user.fullname", fullname),
	)

	query := "UPDATE users SET fullname = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, fullname, updateDatetime, updateUserId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update fullname", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateBio(ctx context.Context, userId uuid.UUID, bio *string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.UpdateBio")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId.String()),
	)

	query := "UPDATE users SET bio = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, bio, updateDatetime, updateUserId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update bio", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// RefreshToken Repository Functions

// CreateRefreshToken creates a new refresh token in the database
func (repository *UserRepository) CreateRefreshToken(ctx context.Context, tx pgx.Tx, refreshToken model.RefreshTokenCreate) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.CreateRefreshToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "INSERT"),
		attribute.String("refreshToken.user_id", refreshToken.UserId.String()),
		attribute.String("refreshToken.token_family", refreshToken.TokenFamily),
	)

	query := `INSERT INTO refresh_tokens
		(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`

	_, err := tx.Exec(ctx, query, refreshToken.Id, refreshToken.UserId, refreshToken.TokenHash, refreshToken.TokenFamily, refreshToken.ExpiresAt, refreshToken.CreatedAt, refreshToken.UpdatedAt, refreshToken.CreatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// CreateRefreshTokenNoTx creates a new refresh token without transaction
func (repository *UserRepository) CreateRefreshTokenNoTx(ctx context.Context, refreshToken model.RefreshToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateRefreshTokenNoTx")
	var err error

	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "INSERT"),
		attribute.String("refreshToken.user_id", refreshToken.UserId),
		attribute.String("refreshToken.token_family", refreshToken.TokenFamily),
	)

	query := `INSERT INTO refresh_tokens
		(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`

	_, err = repository.DB.Exec(ctx, query, refreshToken.Id, refreshToken.UserId, refreshToken.TokenHash, refreshToken.TokenFamily, refreshToken.ExpiresAt, refreshToken.CreatedAt, refreshToken.UpdatedAt, refreshToken.CreatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token without transaction", zap.Error(err))
		return err
	}

	return nil
}

// GetRefreshTokenByHash retrieves a refresh token by its hash
func (repository *UserRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetRefreshTokenByHash")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
	)

	query := `SELECT id, user_id, token_hash, token_family, expires_at, revoked_at, created_at, updated_at, created_by, updated_by
		FROM refresh_tokens
		WHERE token_hash = $1
		LIMIT 1`

	var token model.RefreshToken
	err := repository.DB.QueryRow(ctx, query, tokenHash).Scan(
		&token.Id, &token.UserId, &token.TokenHash, &token.TokenFamily, &token.ExpiresAt, &token.RevokedAt,
		&token.CreatedAt, &token.UpdatedAt, &token.CreatedBy, &token.UpdatedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return token, &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_ERROR,
				Message: "Refresh token is not found",
				Param:   "refreshToken",
			}
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get refresh token by hash", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	return token, nil
}

// GetActiveRefreshTokenByUserIdAndFamily retrieves an active (non-revoked, non-expired) refresh token for a user
func (repository *UserRepository) GetActiveRefreshTokenByUserIdAndFamily(ctx context.Context, userId uuid.UUID, tokenFamily string) (model.RefreshToken, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.GetActiveRefreshTokenByUserIdAndFamily")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "SELECT"),
		attribute.String("refreshToken.user_id", userId.String()),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `SELECT id, user_id, token_hash, token_family, expires_at, revoked_at, created_at, updated_at, created_by, updated_by
		FROM refresh_tokens
		WHERE user_id = $1 AND token_family = $2 AND revoked_at IS NULL AND expires_at > NOW()
		LIMIT 1`

	var token model.RefreshToken
	err := repository.DB.QueryRow(ctx, query, userId, tokenFamily).Scan(
		&token.Id, &token.UserId, &token.TokenHash, &token.TokenFamily, &token.ExpiresAt, &token.RevokedAt,
		&token.CreatedAt, &token.UpdatedAt, &token.CreatedBy, &token.UpdatedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return token, &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_ERROR,
				Message: "Active refresh token is not found",
				Param:   "refreshToken",
			}
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get active refresh token by user and family", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return token, err
	}

	return token, nil
}

// RevokeRefreshTokensByFamily revokes all refresh tokens in a token family (for token rotation)
func (repository *UserRepository) RevokeRefreshTokensByFamily(ctx context.Context, tx pgx.Tx, userId uuid.UUID, tokenFamily string, revokedAt time.Time, updatedAt time.Time, updatedBy uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RevokeRefreshTokensByFamily")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.user_id", userId.String()),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND token_family = $5 AND revoked_at IS NULL`

	result, err := tx.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId, tokenFamily)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh tokens by family", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rowsAffected := result.RowsAffected()
	span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))

	return nil
}

// RevokeRefreshTokensByFamilyNoTx revokes all refresh tokens in a token family without transaction
func (repository *UserRepository) RevokeRefreshTokensByFamilyNoTx(ctx context.Context, userId uuid.UUID, tokenFamily string, revokedAt time.Time, updatedAt time.Time, updatedBy uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RevokeRefreshTokensByFamilyNoTx")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.user_id", userId.String()),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND token_family = $5 AND revoked_at IS NULL`

	result, err := repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId, tokenFamily)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh tokens by family without transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rowsAffected := result.RowsAffected()
	span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))

	return nil
}

// RevokeRefreshTokenById revokes a specific refresh token by ID
func (repository *UserRepository) RevokeRefreshTokenById(ctx context.Context, tx pgx.Tx, tokenId uuid.UUID, revokedAt time.Time, updatedAt time.Time, updatedBy uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RevokeRefreshTokenById")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.id", tokenId.String()),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE id = $4 AND revoked_at IS NULL`

	result, err := tx.Exec(ctx, query, revokedAt, updatedAt, updatedBy, tokenId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh token by ID", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rowsAffected := result.RowsAffected()
	span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))

	return nil
}

// RevokeRefreshTokenByIdNoTx revokes a specific refresh token by ID without transaction
func (repository *UserRepository) RevokeRefreshTokenByIdNoTx(ctx context.Context, tokenId uuid.UUID, revokedAt time.Time, updatedAt time.Time, updatedBy uuid.UUID) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-repository")
	ctx, span := tr.Start(ctx, "repository.RevokeRefreshTokenByIdNoTx")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", repository.Config.String("DB_SYSTEM")),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.id", tokenId.String()),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE id = $4 AND revoked_at IS NULL`

	result, err := repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, tokenId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh token by ID without transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	rowsAffected := result.RowsAffected()
	span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))

	return nil
}

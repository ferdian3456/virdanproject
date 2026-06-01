package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/minio/minio-go/v7"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/attribute"
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

// =============================================================================
// Postgres — Users
// =============================================================================

// Register inserts a new user row (no fullname/bio/avatar — multi-identity refactor).
func (repository *UserRepository) Register(ctx context.Context, tx pgx.Tx, user model.User) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.Register")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("user.id", user.Id),
	)

	query := `INSERT INTO users (id, email, password, settings, created_at, updated_at, created_by, updated_by)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(ctx, query,
		user.Id, user.Email, user.Password, user.Settings,
		user.CreatedAt, user.UpdatedAt, user.CreatedBy, user.UpdatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			err = &model.ConflictError{
				Code:    constant.ERR_CONFLICT_CODE,
				Message: "Email already exists",
				Param:   "email",
			}
			return err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to register user", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) CheckEmailUnique(ctx context.Context, email string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckEmailUnique")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.email", email),
	)

	query := `SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1`

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

func (repository *UserRepository) GetUserAuthByEmail(ctx context.Context, email string) (string, string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserAuthByEmail")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.email", email),
	)

	query := `SELECT id, password FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1`

	var id, passwordHash string
	err = repository.DB.QueryRow(ctx, query, email).Scan(&id, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.BadRequestError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Email is not found",
				Param:   "email",
			}
			return "", "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user auth by email", zap.Error(err))
		return "", "", err
	}

	return id, passwordHash, nil
}

func (repository *UserRepository) GetUserInfo(ctx context.Context, id string) (model.UserResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserInfo")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", id),
	)

	query := `SELECT id, email, settings, created_at, updated_at FROM users
			  WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var resp model.UserResponse
	err = repository.DB.QueryRow(ctx, query, id).Scan(
		&resp.Id, &resp.Email, &resp.Settings, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return resp, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user info by ID", zap.Error(err))
		return resp, err
	}

	return resp, nil
}

// CheckUserActive returns true if user exists and is not soft-deleted.
func (repository *UserRepository) CheckUserActive(ctx context.Context, userId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckUserActive")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
	)

	query := `SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var exists int
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check user active status", zap.Error(err))
		return false, err
	}

	return true, nil
}

// SoftDeleteUser sets deleted_at to mark account closed. Operates inside a transaction.
func (repository *UserRepository) SoftDeleteUser(ctx context.Context, tx pgx.Tx, userId string, now time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SoftDeleteUser")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)

	query := `UPDATE users SET deleted_at = $1, updated_at = $2, updated_by = $3
			  WHERE id = $4 AND deleted_at IS NULL`

	_, err = tx.Exec(ctx, query, now, now, userId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to soft delete user", zap.Error(err))
		return err
	}

	return nil
}

// =============================================================================
// Redis — Access token cache
// =============================================================================

func (repository *UserRepository) SetAccessTokenInCache(ctx context.Context, accessToken string, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "SET"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)
	hashedAccessToken := util.HashToken(accessToken)

	err = repository.DBCache.Set(ctx, accessTokenKey, hashedAccessToken, util.AccessTokenDuration).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set access token in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetAccessTokenInCache(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "GET"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	hashedToken, err := repository.DBCache.Get(ctx, accessTokenKey).Result()
	if errors.Is(err, redis.Nil) {
		err = &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_CODE,
			Message: "Authorization token not found or expired",
			Param:   "accessToken",
		}
		return "", err
	} else if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get access token from cache", zap.Error(err))
		return "", err
	}

	return hashedToken, nil
}

func (repository *UserRepository) RemoveAuthToken(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RemoveAuthToken")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	err = repository.DBCache.Del(ctx, accessTokenKey).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to remove access token from cache", zap.Error(err))
		return err
	}

	return nil
}

// RemoveAllAccessTokensFromCache clears all access token entries cached for the user.
// Current key scheme `auth:accessToken:{userId}` is a single key per user, so DEL suffices.
func (repository *UserRepository) RemoveAllAccessTokensFromCache(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RemoveAllAccessTokensFromCache")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	err = repository.DBCache.Del(ctx, accessTokenKey).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to clear access token cache", zap.Error(err))
		return err
	}

	return nil
}

// =============================================================================
// Redis — Signup session
// =============================================================================

func (repository *UserRepository) SetSignupSession(ctx context.Context, sessionId string, email string, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetSignupSession")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId),
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
			util.RecordErrorTelemetry(ctx, span, err)
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

func (repository *UserRepository) GetOTPSignupSessionData(ctx context.Context, sessionId string) (model.OTPSignupData, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOTPSignupSessionData")
	var err error
	var otp model.OTPSignupData

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "otp", "otp_expires_at").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP signup session data", zap.Error(err))
		return otp, err
	}

	if vals[0] == nil || vals[1] == nil {
		return otp, nil
	}

	otp.OTP = vals[0].(string)
	expiresAt, parseErr := strconv.ParseInt(vals[1].(string), 10, 64)
	if parseErr != nil {
		err = parseErr
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to parse otp expires at", zap.Error(err))
		return otp, err
	}
	otp.ExpiresAt = expiresAt

	return otp, nil
}

func (repository *UserRepository) GetOtpDataForResend(ctx context.Context, sessionId string) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOtpDataForResend")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "email", "otp_expires_at").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP session data for resend", zap.Error(err))
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) GetSignupState(ctx context.Context, sessionId string) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetSignupState")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "step").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get signup state from cache", zap.Error(err))
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) DeleteOTPState(ctx context.Context, sessionId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteOTPState")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HDEL"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err = repository.DBCache.HDel(ctx, key, "otp", "otp_expires_at").Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete OTP state from cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateSessionForResendOtp(ctx context.Context, sessionId string, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateSessionForResendOtp")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err = repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"otp":            otp,
		"otp_expires_at": otpExpiresAt,
	}).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update session for resend otp", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) SetVerificationOTPState(ctx context.Context, sessionId string, verifiedAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetVerificationOTPState")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	err = repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"step":            model.SignupStepOTPVerified,
		"otp_verified_at": verifiedAt,
	}).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set verification OTP state in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetAllSessionData(ctx context.Context, sessionId string) (map[string]string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetAllSessionData")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HGETALL"),
		attribute.String("signup.session_id", sessionId),
	)

	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HGetAll(ctx, key).Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get all signup session data", zap.Error(err))
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
			util.RecordErrorTelemetry(ctx, span, err)
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
	if errors.Is(err, redis.Nil) {
		return false, "", nil
	} else if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check signup email session", zap.Error(err))
		return false, "", err
	}

	return true, sessionId, nil
}

func (repository *UserRepository) DeleteSignupSession(ctx context.Context, sessionId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteSignupSession")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
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
		return err
	}

	return nil
}

func (repository *UserRepository) DeleteEmailSignupSession(ctx context.Context, email string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteEmailSignupSession")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.email", email),
	)

	key := fmt.Sprintf("signup_email:%s", email)

	err = repository.DBCache.Del(ctx, key).Err()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete email signup session", zap.Error(err))
		return err
	}

	return nil
}

// =============================================================================
// Postgres — Refresh tokens
// =============================================================================

func (repository *UserRepository) CreateRefreshToken(ctx context.Context, tx pgx.Tx, refreshToken model.RefreshToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateRefreshToken")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("refreshToken.user_id", refreshToken.UserId),
		attribute.String("refreshToken.token_family", refreshToken.TokenFamily),
	)

	query := `INSERT INTO refresh_tokens
		(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		refreshToken.Id, refreshToken.UserId, refreshToken.TokenHash, refreshToken.TokenFamily,
		refreshToken.ExpiresAt, refreshToken.CreatedAt, refreshToken.UpdatedAt,
		refreshToken.CreatedBy, refreshToken.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) CreateRefreshTokenNoTx(ctx context.Context, refreshToken model.RefreshToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateRefreshTokenNoTx")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("refreshToken.user_id", refreshToken.UserId),
		attribute.String("refreshToken.token_family", refreshToken.TokenFamily),
	)

	query := `INSERT INTO refresh_tokens
		(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = repository.DB.Exec(ctx, query,
		refreshToken.Id, refreshToken.UserId, refreshToken.TokenHash, refreshToken.TokenFamily,
		refreshToken.ExpiresAt, refreshToken.CreatedAt, refreshToken.UpdatedAt,
		refreshToken.CreatedBy, refreshToken.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetRefreshTokenByHash")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
	)

	query := `SELECT id, user_id, token_hash, token_family, expires_at, revoked_at,
			  created_at, updated_at, created_by, updated_by
			  FROM refresh_tokens WHERE token_hash = $1 LIMIT 1`

	var token model.RefreshToken
	err = repository.DB.QueryRow(ctx, query, tokenHash).Scan(
		&token.Id, &token.UserId, &token.TokenHash, &token.TokenFamily,
		&token.ExpiresAt, &token.RevokedAt,
		&token.CreatedAt, &token.UpdatedAt, &token.CreatedBy, &token.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "Refresh token is not found",
				Param:   "refreshToken",
			}
			return token, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get refresh token by hash", zap.Error(err))
		return token, err
	}

	return token, nil
}

func (repository *UserRepository) GetActiveRefreshTokenByUserIdAndFamily(ctx context.Context, userId string, tokenFamily string) (model.RefreshToken, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetActiveRefreshTokenByUserIdAndFamily")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("refreshToken.user_id", userId),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `SELECT id, user_id, token_hash, token_family, expires_at, revoked_at,
			  created_at, updated_at, created_by, updated_by
			  FROM refresh_tokens
			  WHERE user_id = $1 AND token_family = $2 AND revoked_at IS NULL AND expires_at > NOW()
			  LIMIT 1`

	var token model.RefreshToken
	err = repository.DB.QueryRow(ctx, query, userId, tokenFamily).Scan(
		&token.Id, &token.UserId, &token.TokenHash, &token.TokenFamily,
		&token.ExpiresAt, &token.RevokedAt,
		&token.CreatedAt, &token.UpdatedAt, &token.CreatedBy, &token.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "Active refresh token is not found",
				Param:   "refreshToken",
			}
			return token, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get active refresh token", zap.Error(err))
		return token, err
	}

	return token, nil
}

func (repository *UserRepository) RevokeRefreshTokensByFamily(ctx context.Context, tx pgx.Tx, userId string, tokenFamily string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeRefreshTokensByFamily")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.user_id", userId),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND token_family = $5 AND revoked_at IS NULL`

	result, err := tx.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId, tokenFamily)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh tokens by family", zap.Error(err))
		return err
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected()))
	return nil
}

func (repository *UserRepository) RevokeRefreshTokensByFamilyNoTx(ctx context.Context, userId string, tokenFamily string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeRefreshTokensByFamilyNoTx")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.user_id", userId),
		attribute.String("refreshToken.token_family", tokenFamily),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND token_family = $5 AND revoked_at IS NULL`

	result, err := repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId, tokenFamily)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh tokens by family", zap.Error(err))
		return err
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected()))
	return nil
}

func (repository *UserRepository) RevokeRefreshTokenById(ctx context.Context, tx pgx.Tx, tokenId string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeRefreshTokenById")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.id", tokenId),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE id = $4 AND revoked_at IS NULL`

	result, err := tx.Exec(ctx, query, revokedAt, updatedAt, updatedBy, tokenId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh token by ID", zap.Error(err))
		return err
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected()))
	return nil
}

func (repository *UserRepository) RevokeRefreshTokenByIdNoTx(ctx context.Context, tokenId string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeRefreshTokenByIdNoTx")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("refreshToken.id", tokenId),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE id = $4 AND revoked_at IS NULL`

	result, err := repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, tokenId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh token by ID", zap.Error(err))
		return err
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected()))
	return nil
}

func (repository *UserRepository) RevokeAllRefreshTokensByUserId(ctx context.Context, userId string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeAllRefreshTokensByUserId")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND revoked_at IS NULL`

	_, err = repository.DB.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke all refresh tokens for user", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) RevokeAllRefreshTokensByUserIdTx(ctx context.Context, tx pgx.Tx, userId string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeAllRefreshTokensByUserIdTx")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)

	query := `UPDATE refresh_tokens
		SET revoked_at = $1, updated_at = $2, updated_by = $3
		WHERE user_id = $4 AND revoked_at IS NULL`

	_, err = tx.Exec(ctx, query, revokedAt, updatedAt, updatedBy, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke all refresh tokens for user", zap.Error(err))
		return err
	}

	return nil
}

func (repository *UserRepository) GetPasswordHashById(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPasswordHashById")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
	)

	query := `SELECT password FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var hash string
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get password hash", zap.Error(err))
		return "", err
	}
	return hash, nil
}

func (repository *UserRepository) UpdatePasswordHash(ctx context.Context, userId, newHash string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdatePasswordHash")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)

	query := `UPDATE users SET password = $1, updated_at = $2, updated_by = $3 WHERE id = $4 AND deleted_at IS NULL`
	_, err = repository.DB.Exec(ctx, query, newHash, updatedAt, userId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update password hash", zap.Error(err))
		return err
	}
	return nil
}

// =============================================================================
// Email change (Redis session + Postgres email update)
// =============================================================================

// SetEmailChangeSession stashes a pending email-change request in Redis. Key
// shape: email_change:{userId} → hash { newEmail, otpHash, attempts="0" }.
// TTL = 10 min. Overwrites any existing pending request.
func (repository *UserRepository) SetEmailChangeSession(ctx context.Context, userId, newEmail, otpHash string, ttl time.Duration) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetEmailChangeSession")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("user.id", userId),
	)

	key := fmt.Sprintf("email_change:%s", userId)
	pipe := repository.DBCache.TxPipeline()
	pipe.Del(ctx, key)
	pipe.HSet(ctx, key, map[string]interface{}{
		"newEmail": newEmail,
		"otpHash":  otpHash,
		"attempts": "0",
	})
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set email change session", zap.Error(err))
		return err
	}
	return nil
}

func (repository *UserRepository) GetEmailChangeSessionTTL(ctx context.Context, userId string) (time.Duration, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetEmailChangeSessionTTL")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "TTL"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	var ttl time.Duration
	ttl, err = repository.DBCache.TTL(ctx, key).Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to read email change session TTL", zap.Error(err))
		return 0, err
	}
	return ttl, nil
}

func (repository *UserRepository) GetEmailChangeSession(ctx context.Context, userId string) (newEmail, otpHash string, attempts int, err error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetEmailChangeSession")
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	var data []interface{}
	data, err = repository.DBCache.HMGet(ctx, key, "newEmail", "otpHash", "attempts").Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get email change session", zap.Error(err))
		return "", "", 0, err
	}
	if data[0] == nil || data[1] == nil {
		return "", "", 0, nil
	}
	newEmail, _ = data[0].(string)
	otpHash, _ = data[1].(string)
	if data[2] != nil {
		if s, ok := data[2].(string); ok {
			if n, convErr := strconv.Atoi(s); convErr == nil {
				attempts = n
			}
		}
	}
	return newEmail, otpHash, attempts, nil
}

func (repository *UserRepository) IncrementEmailChangeAttempts(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.IncrementEmailChangeAttempts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HINCRBY"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	_, err = repository.DBCache.HIncrBy(ctx, key, "attempts", 1).Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to bump email change attempts", zap.Error(err))
		return err
	}
	return nil
}

func (repository *UserRepository) DeleteEmailChangeSession(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteEmailChangeSession")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	_, err = repository.DBCache.Del(ctx, key).Result()
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete email change session", zap.Error(err))
		return err
	}
	return nil
}

func (repository *UserRepository) UpdateEmail(ctx context.Context, userId, newEmail string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateEmail")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)
	query := `UPDATE users SET email = $1, updated_at = $2, updated_by = $3 WHERE id = $4 AND deleted_at IS NULL`
	_, err = repository.DB.Exec(ctx, query, newEmail, updatedAt, userId, userId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			err = &model.ConflictError{
				Code:    constant.ERR_CONFLICT_CODE,
				Message: "Email already in use",
				Param:   "newEmail",
			}
			return err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update user email", zap.Error(err))
		return err
	}
	return nil
}

func (repository *UserRepository) GetUserEmail(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserEmail")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
	)
	query := `SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`
	var email string
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user email", zap.Error(err))
		return "", err
	}
	return email, nil
}

// UpdateNotificationPrefs merges only the notif_* keys into users.settings via jsonb_build_object,
// leaving other settings keys untouched. updatedAt is passed in from the usecase (time from Go).
func (repository *UserRepository) UpdateNotificationPrefs(ctx context.Context, userId string, prefs model.NotificationPrefs, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateNotificationPrefs")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", userId),
	)

	query := `UPDATE users
	          SET settings = settings || jsonb_build_object(
	              'notif_like',    $1::boolean,
	              'notif_comment', $2::boolean,
	              'notif_reply',   $3::boolean
	          ),
	          updated_at = $4,
	          updated_by = $5
	          WHERE id = $6 AND deleted_at IS NULL`

	_, err = repository.DB.Exec(ctx, query,
		prefs.NotifLike, prefs.NotifComment, prefs.NotifReply,
		updatedAt, userId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update notification prefs", zap.Error(err))
		return err
	}
	return nil
}

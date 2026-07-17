package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ferdian3456/virdanproject/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type Repository struct {
	Log     *zap.Logger
	Config  *koanf.Koanf
	DB      *pgxpool.Pool
	DBCache *redis.Client
}

func NewRepository(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool, dbCache *redis.Client) *Repository {
	return &Repository{
		Log:     log,
		Config:  config,
		DB:      db,
		DBCache: dbCache,
	}
}

func (repository *Repository) Register(ctx context.Context, tx pgx.Tx, user User) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.Register")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
			err = &shared.ConflictError{
				Code:    shared.ERR_CONFLICT_CODE,
				Message: "Email already exists",
				Param:   "email",
			}
			return err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to register user", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CheckEmailUnique(ctx context.Context, email string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckEmailUnique")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check email uniqueness", zap.Error(err))
		return false, err
	}

	return true, nil
}

func (repository *Repository) GetUserAuthByEmail(ctx context.Context, email string) (string, string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserAuthByEmail")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
			err = &shared.BadRequestError{
				Code:    shared.ERR_VALIDATION_CODE,
				Message: "Email is not found",
				Param:   "email",
			}
			return "", "", err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user auth by email", zap.Error(err))
		return "", "", err
	}

	return id, passwordHash, nil
}

func (repository *Repository) SetAccessTokenInCache(ctx context.Context, accessToken string, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "SET"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)
	hashedAccessToken := shared.HashToken(accessToken)

	err = repository.DBCache.Set(ctx, accessTokenKey, hashedAccessToken, shared.AccessTokenDuration).Err()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set access token in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetAccessTokenInCache(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetAccessTokenInCache")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		err = &shared.UnauthorizedError{
			Code:    shared.ERR_UNAUTHORIZED_CODE,
			Message: "Authorization token not found or expired",
			Param:   "accessToken",
		}
		return "", err
	} else if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get access token from cache", zap.Error(err))
		return "", err
	}

	return hashedToken, nil
}

func (repository *Repository) RemoveAuthToken(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RemoveAuthToken")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to remove access token from cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) SetSignupSession(ctx context.Context, sessionId string, email string, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetSignupSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		"step":           SignupStepStart,
		"created_at":     time.Now().Unix(),
	}).Err()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set signup session in cache", zap.Error(err))
		return err
	}

	err = repository.DBCache.Expire(ctx, key, 30*time.Minute).Err()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set expiration for signup session", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) SetSignupEmailSession(ctx context.Context, sessionId string, email string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetSignupEmailSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set signup email session in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetOTPSignupSessionData(ctx context.Context, sessionId string) (OTPSignupData, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOTPSignupSessionData")
	var err error
	var otp OTPSignupData

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP signup session data", zap.Error(err))
		return otp, err
	}

	if vals[0] == nil || vals[1] == nil {
		return otp, nil
	}

	otp.OTP = vals[0].(string)
	expiresAt, parseErr := strconv.ParseInt(vals[1].(string), 10, 64)
	if parseErr != nil {
		err = parseErr
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to parse otp expires at", zap.Error(err))
		return otp, err
	}
	otp.ExpiresAt = expiresAt

	return otp, nil
}

func (repository *Repository) GetOtpDataForResend(ctx context.Context, sessionId string) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOtpDataForResend")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get OTP session data for resend", zap.Error(err))
		return vals, err
	}

	return vals, nil
}

func (repository *Repository) GetSignupState(ctx context.Context, sessionId string) ([]interface{}, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetSignupState")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get signup state from cache", zap.Error(err))
		return vals, err
	}

	return vals, nil
}

func (repository *Repository) DeleteOTPState(ctx context.Context, sessionId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteOTPState")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete OTP state from cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateSessionForResendOtp(ctx context.Context, sessionId string, otp string, otpExpiresAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateSessionForResendOtp")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update session for resend otp", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) SetVerificationOTPState(ctx context.Context, sessionId string, verifiedAt int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetVerificationOTPState")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		"step":            SignupStepOTPVerified,
		"otp_verified_at": verifiedAt,
	}).Err()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set verification OTP state in cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetAllSessionData(ctx context.Context, sessionId string) (map[string]string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetAllSessionData")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get all signup session data", zap.Error(err))
		return nil, err
	}

	return vals, nil
}

func (repository *Repository) CheckSignupEmailSession(ctx context.Context, email string) (bool, string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckSignupEmailSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check signup email session", zap.Error(err))
		return false, "", err
	}

	return true, sessionId, nil
}

func (repository *Repository) DeleteSignupSession(ctx context.Context, sessionId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteSignupSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete signup session", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteEmailSignupSession(ctx context.Context, email string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteEmailSignupSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete email signup session", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CreateRefreshToken(ctx context.Context, tx pgx.Tx, refreshToken RefreshToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateRefreshToken")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CreateRefreshTokenNoTx(ctx context.Context, refreshToken RefreshToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateRefreshTokenNoTx")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create refresh token", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetRefreshTokenByHash")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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

	var token RefreshToken
	err = repository.DB.QueryRow(ctx, query, tokenHash).Scan(
		&token.Id, &token.UserId, &token.TokenHash, &token.TokenFamily,
		&token.ExpiresAt, &token.RevokedAt,
		&token.CreatedAt, &token.UpdatedAt, &token.CreatedBy, &token.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "Refresh token is not found",
				Param:   "refreshToken",
			}
			return token, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get refresh token by hash", zap.Error(err))
		return token, err
	}

	return token, nil
}

func (repository *Repository) RevokeRefreshTokensByFamily(ctx context.Context, tx pgx.Tx, userId string, tokenFamily string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeRefreshTokensByFamily")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke refresh tokens by family", zap.Error(err))
		return err
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected()))
	return nil
}

func (repository *Repository) RevokeAllRefreshTokensByUserId(ctx context.Context, userId string, revokedAt time.Time, updatedAt time.Time, updatedBy string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RevokeAllRefreshTokensByUserId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to revoke all refresh tokens for user", zap.Error(err))
		return err
	}

	return nil
}

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
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type UserRepository struct {
	Log      *zap.Logger
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewUserRepository(zap *zap.Logger, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *UserRepository {
	return &UserRepository{
		Log:      zap,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

// Postgresql
func (repository *UserRepository) Register(ctx context.Context, tx pgx.Tx, user model.User) error {
	query := "INSERT INTO users (id,username,fullname,bio,avatar_image_id, email,password, settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"

	_, err := tx.Exec(ctx, query, user.Id, user.Username, user.Fullname, user.Bio, user.AvatarImageId, user.Email, user.Password, user.Settings, user.CreateDatetime, user.UpdateDatetime, user.CreateUserId, user.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

// Postgresql
func (repository *UserRepository) RegisterNoTx(ctx context.Context, user model.User) error {
	query := "INSERT INTO users (id,username,fullname,bio,avatar_image_id, email, password, settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"

	_, err := repository.DB.Exec(ctx, query, user.Id, user.Username, user.Fullname, user.Bio, user.AvatarImageId, user.Email, user.Password, user.Settings, user.CreateDatetime, user.UpdateDatetime, user.CreateUserId, user.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) CheckUsernameOrEmailUnique(ctx context.Context, username string, email string) (string, string, error) {
	query := "SELECT username,email FROM users WHERE username=$1 OR email=$2 LIMIT 1"

	var existUsername string
	var existEmail string
	err := repository.DB.QueryRow(ctx, query, username, email).Scan(&existUsername, &existEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return existUsername, existEmail, nil
		}
		return existUsername, existEmail, err
	}

	return existUsername, existEmail, nil
}

func (repository *UserRepository) GetUserAuth(ctx context.Context, username string) (uuid.UUID, string, error) {
	query := "SELECT id,password FROM users WHERE username=$1 LIMIT 1"

	var id uuid.UUID
	var passwordHash string

	err := repository.DB.QueryRow(ctx, query, username).Scan(&id, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return id, passwordHash, &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Username is not found",
				Param:   "username",
			}
		}
		return id, passwordHash, err
	}

	return id, passwordHash, nil
}

func (repository *UserRepository) GetUserInfo(ctx context.Context, id uuid.UUID) (model.UserResponse, error) {
	query := `SELECT A.id,A.username,A.fullname,A.email,B.object_key,A.create_datetime,A.update_datetime
			FROM users A
			LEFT JOIN user_avatar_images B ON A.id = B.user_id
			WHERE A.id=$1
			LIMIT 1`

	user := model.UserResponse{}
	err := repository.DB.QueryRow(ctx, query, id).Scan(&user.Id, &user.Username, &user.Fullname, &user.Email, &user.AvatarImage, &user.CreateDatetime, &user.UpdateDatetime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, &model.ValidationError{
				Code:    constant.ERR_NOT_FOUND_ERROR,
				Message: "User not found",
				Param:   "userId",
			}
		}
		return user, err
	}

	return user, nil
}

// Redis - Cache
func (repository *UserRepository) SetAuthTokenInCache(ctx context.Context, accessToken string, refreeshToken string, userId uuid.UUID) error {
	accessTokenKey := fmt.Sprintf("auth:acccessToken:%s", userId)
	refreshTokenKey := fmt.Sprintf("auth:refreshToken:%s", userId)

	// Hash tokens before storing in Redis for security
	hashedAccessToken := util.HashToken(accessToken)
	hashedRefreshToken := util.HashToken(refreeshToken)

	err := repository.DBCache.Set(ctx, accessTokenKey, hashedAccessToken, 15*time.Minute).Err()
	if err != nil {
		return err
	}

	err = repository.DBCache.Set(ctx, refreshTokenKey, hashedRefreshToken, 15*time.Minute).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) GetAccessTokenInCache(ctx context.Context, userId uuid.UUID) (string, error) {
	accessTokenKey := fmt.Sprintf("auth:acccessToken:%s", userId)
	hashedToken, err := repository.DBCache.Get(ctx, accessTokenKey).Result()
	if err == redis.Nil {
		return hashedToken, &model.ValidationError{
			Code:    constant.ERR_NOT_FOUND_ERROR,
			Message: "Authorization token not found or expired",
			Param:   "accessToken",
		}
	} else if err != nil {
		return hashedToken, err
	}

	return hashedToken, nil
}

func (repository *UserRepository) RemoveAuthToken(ctx context.Context, userId uuid.UUID) error {
	accessTokenKey := fmt.Sprintf("auth:acccessToken:%s", userId)
	refreshTokenKey := fmt.Sprintf("auth:refreshToken:%s", userId)

	err := repository.DBCache.Del(ctx, accessTokenKey).Err()
	if err != nil {
		return err
	}

	err = repository.DBCache.Del(ctx, refreshTokenKey).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) UploadUserAvatar(ctx context.Context, bucketName string, imageName string, imageFile *bytes.Reader, imageSize int64) error {
	_, err := repository.DBObject.PutObject(ctx, bucketName, imageName, imageFile, imageSize,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) GetUserAvatar(ctx context.Context, tx pgx.Tx, userId uuid.UUID) (string, error) {
	query := "SELECT object_key FROM user_avatar_images WHERE user_id=$1 LIMIT 1"

	var objectKey string
	err := tx.QueryRow(ctx, query, userId).Scan(&objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return objectKey, err
	}

	return objectKey, nil
}

func (repository *UserRepository) DeleteUserAvatar(ctx context.Context, bucketName string, fileName string) error {
	err := repository.DBObject.RemoveObject(ctx, bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) DeleteAvatarImage(ctx context.Context, tx pgx.Tx, userId uuid.UUID) error {
	query := "DELETE FROM user_avatar_images WHERE user_id=$1"

	_, err := tx.Exec(ctx, query, userId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) AddUserAvatar(ctx context.Context, tx pgx.Tx, avatar model.UserAvatarImage) error {
	query := "INSERT INTO user_avatar_images (id, user_id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"

	_, err := tx.Exec(ctx, query, avatar.Id, avatar.UserId, avatar.Bucket, avatar.ObjectKey, avatar.MimeType, avatar.Size, avatar.CreateDatetime, avatar.UpdateDatetime, avatar.CreateUserId, avatar.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) SetSignupSession(ctx context.Context, sessionId uuid.UUID, email string, otp string, otpExpiresAt int64) error {
	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"email":          email,
		"otp":            otp,
		"otp_expires_at": otpExpiresAt,
		"step":           model.SignupStepStart,
		"create_at":      time.Now().Unix(),
	}).Err()

	if err != nil {
		return err
	}

	err = repository.DBCache.Expire(ctx, key, 30*time.Minute).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) SetSignupEmailSession(ctx context.Context, sessionId string, email string) error {
	key := fmt.Sprintf("signup_email:%s", email)

	err := repository.DBCache.Set(ctx, key, sessionId, 30*time.Minute).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) GetOTPSignupSessionData(ctx context.Context, sessionId uuid.UUID) ([]interface{}, error) {
	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "otp", "otp_expires_at").Result()
	if err != nil {
		return vals, err
	}

	return vals, nil
}
func (repository *UserRepository) GetSignupState(ctx context.Context, sessionId uuid.UUID) ([]interface{}, error) {
	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HMGet(ctx, key, "step").Result()
	if err != nil {
		return vals, err
	}

	return vals, nil
}

func (repository *UserRepository) DeleteOTPState(ctx context.Context, sessionId uuid.UUID) error {
	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HDel(ctx, key, "otp", "otp_expires_at").Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) SetVerificationOTPState(ctx context.Context, sessionId uuid.UUID, verifiedAt int64) error {
	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"step":            model.SignupStepOTPVerified,
		"otp_verified_at": verifiedAt,
	}).Err()

	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) CheckUsernameUnique(ctx context.Context, username string) (int, error) {
	query := "SELECT 1 FROM users WHERE username=$1 LIMIT 1"

	var exists int
	err := repository.DB.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}
		return exists, err
	}

	return exists, nil
}

func (repository *UserRepository) CheckEmailUnique(ctx context.Context, email string) (int, error) {
	query := "SELECT 1 FROM users WHERE email=$1 LIMIT 1"

	var exists int
	err := repository.DB.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}
		return exists, err
	}

	return exists, nil
}

func (repository *UserRepository) SetVerificationUsernameState(ctx context.Context, sessionId uuid.UUID, username string) error {
	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.HSet(ctx, key, map[string]interface{}{
		"step":     model.SignupStepUsernameSet,
		"username": username,
	}).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) GetAllSessionData(ctx context.Context, sessionId uuid.UUID) (map[string]string, error) {
	key := fmt.Sprintf("signup:%s", sessionId)

	vals, err := repository.DBCache.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return vals, nil
}

func (repository *UserRepository) CheckSignupEmailSession(ctx context.Context, email string) (bool, string, error) {
	key := fmt.Sprintf("signup_email:%s", email)
	sessionId, err := repository.DBCache.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, sessionId, nil
	} else if err != nil {
		return false, sessionId, err
	}

	return true, sessionId, nil
}

func (repository *UserRepository) DeleteSignupSession(ctx context.Context, sessionId string) error {
	key := fmt.Sprintf("signup:%s", sessionId)

	err := repository.DBCache.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) DeleteEmailSignupSession(ctx context.Context, sesisonId string) error {
	key := fmt.Sprintf("signup_email:%s", sesisonId)

	err := repository.DBCache.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateUsername(ctx context.Context, userId uuid.UUID, username string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE users SET username = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, username, updateDatetime, updateUserId, userId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateFullname(ctx context.Context, userId uuid.UUID, fullname string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE users SET fullname = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, fullname, updateDatetime, updateUserId, userId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *UserRepository) UpdateBio(ctx context.Context, userId uuid.UUID, bio *string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE users SET bio = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, bio, updateDatetime, updateUserId, userId)
	if err != nil {
		return err
	}

	return nil
}

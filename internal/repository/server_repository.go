package repository

import (
	"context"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ServerRepository struct {
	Log     *zap.Logger
	DB      *pgxpool.Pool
	DBCache *redis.Client
}

func NewServerRepository(zap *zap.Logger, db *pgxpool.Pool, dbCache *redis.Client) *ServerRepository {
	return &ServerRepository{
		Log:     zap,
		DB:      db,
		DBCache: dbCache,
	}
}

func (repository *ServerRepository) CreateServer(ctx context.Context, tx pgx.Tx, server model.Server) error {
	query := "INSERT INTO servers (id,owner_id,name,short_name,category_id,avatar_image_id, banner_image_id, description,settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)"

	_, err := tx.Exec(ctx, query, server.Id, server.OwnerId, server.Name, server.ShortName, server.CategoryId, server.AvatarImageId, server.BannerImageId, server.Description, server.Settings, server.CreateDatetime, server.UpdateDatetime, server.CreateUserId, server.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateRole(ctx context.Context, tx pgx.Tx, serverRole model.ServerRole) error {
	query := "INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)"

	_, err := tx.Exec(ctx, query, serverRole.Id, serverRole.ServerId, serverRole.Name, serverRole.Permissions, serverRole.CreateDatetime, serverRole.UpdateDatetime, serverRole.CreateUserId, serverRole.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerMember(ctx context.Context, tx pgx.Tx, serverMember model.ServerMember) error {
	query := "INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_at, left_at, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)"

	_, err := tx.Exec(ctx, query, serverMember.Id, serverMember.ServerId, serverMember.UserId, serverMember.ServerRoleId, serverMember.Status, serverMember.JoinedAt, serverMember.LeftAt, serverMember.CreateDatetime, serverMember.UpdateDatetime, serverMember.CreateUserId, serverMember.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerMemberProfile(ctx context.Context, tx pgx.Tx, serverMemberProfile model.ServerMemberProfile) error {
	query := "INSERT INTO server_member_profiles (id, server_member_id, server_id, user_id, username, fullname, bio, avatar_image_id, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"

	_, err := tx.Exec(ctx, query, serverMemberProfile.Id, serverMemberProfile.ServerMemberId, serverMemberProfile.ServerId, serverMemberProfile.UserId, serverMemberProfile.Username, serverMemberProfile.Fullname, serverMemberProfile.Bio, serverMemberProfile.AvatarImageId, serverMemberProfile.CreateDatetime, serverMemberProfile.UpdateDatetime, serverMemberProfile.CreateUserId, serverMemberProfile.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

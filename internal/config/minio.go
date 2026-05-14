package config

import (
	"context"

	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

func NewMinIO(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *minio.Client {
	endpoint := config.String("MINIO_INTERNAL_URL")
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.String("MINIO_USER"), config.String("MINIO_PASSWORD"), ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal("Failed to initialize minio client", zap.Error(err))
	}

	bucketName := config.String("MINIO_BUCKET_NAME")
	location := config.String("MINIO_LOCATION")

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: location,
	})

	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Info("MinIO bucket already exists", zap.String("bucket", bucketName))
		} else {
			log.Fatal("Failed to create minio bucket", zap.Error(err), zap.String("bucket", bucketName))
		}
	} else {
		log.Info("Successfully created minio bucket", zap.String("bucket", bucketName))
	}

	log.Info("MinIO client initialized",
		zap.String("endpoint", endpoint),
		zap.String("bucket", bucketName),
		zap.String("location", location),
	)

	return minioClient
}

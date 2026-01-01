package config

import (
	"context"

	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

func NewMinIO(config *koanf.Koanf, log *zap.Logger) *minio.Client {
	minioClient, err := minio.New(config.String("MINIO_URL"), &minio.Options{
		Creds:  credentials.NewStaticV4(config.String("MINIO_USER"), config.String("MINIO_PASSWORD"), ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal("failed to initialize minio client", zap.Error(err))
	}

	bucketName := config.String("MINIO_BUCKET_NAME")
	location := config.String("MINIO_LOCATION")
	ctx := context.Background()

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: location,
	})

	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Info("Minio bucket already exists")
		} else {
			log.Fatal("Failed to create minio bucket", zap.Error(err))
		}
	} else {
		log.Info("Successfully created minio bucket")
	}

	return minioClient
}

// Optimized upload with parallel processing
//func (m *MinIOClient) UploadObject(ctx context.Context, bucketName, objectName string, data []byte) error {
//	// Use context with timeout for extreme performance
//	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
//	defer cancel()
//
//	// Upload with optimized settings
//	_, err := m.Client.PutObject(ctx, bucketName, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
//		ContentType: "application/octet-stream",
//		// Enable parallel upload for maximum speed
//		NumThreads: 8,
//		// Use optimized part size for MinIO
//		PartSize: 64 * 1024 * 1024, // 64MB parts
//	})
//
//	if err != nil {
//		m.Log.Error("Failed to upload object to MinIO",
//			zap.String("bucket", bucketName),
//			zap.String("object", objectName),
//			zap.Error(err))
//		return err
//	}
//
//	m.Log.Info("Successfully uploaded object to MinIO",
//		zap.String("bucket", bucketName),
//		zap.String("object", objectName),
//		zap.Int64("size", int64(len(data))))
//
//	return nil
//}

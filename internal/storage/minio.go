package storage

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	Client     *minio.Client
	BucketName string
}

// NewMinioClient — инициализация клиента
func NewMinioClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioClient, error) {
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init minio client: %w", err)
	}

	// ctx := context.Background()

	// Проверяем наличие бакета, если нет — создаём
	// exists, err := cli.BucketExists(ctx, bucket)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	// }
	// if !exists {
	// 	if err := cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
	// 		return nil, fmt.Errorf("failed to create bucket %s: %w", bucket, err)
	// 	}
	// }

	return &MinioClient{
		Client:     cli,
		BucketName: bucket,
	}, nil
}

package repository

import (
	"context"
	"io"
	"time"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/storage"

	"github.com/minio/minio-go/v7"
)

type StorageRepo struct {
	minio *storage.MinioClient
}

func NewStorageRepo(min *storage.MinioClient) *StorageRepo {
	return &StorageRepo{
		minio: min,
	}
}

// UploadFile — загружает в MinIO, возвращает путь файла

func (s *StorageRepo) Ping(ctx context.Context) error {
	if s == nil || s.minio == nil || s.minio.Client == nil {
		logger.Error("storage ping failed", nil, "component", "repository", "repository", "storage", "method", "Ping", "external_system", "minio", "reason", "storage_not_configured")
		return errStorageNotConfigured
	}

	logger.Info("storage ping started", "component", "repository", "repository", "storage", "method", "Ping", "external_system", "minio", "bucket", s.minio.BucketName)
	_, err := s.minio.Client.BucketExists(ctx, s.minio.BucketName)
	if err != nil {
		logger.Error("storage ping failed", err, "component", "repository", "repository", "storage", "method", "Ping", "external_system", "minio", "bucket", s.minio.BucketName)
		return err
	}
	logger.Info("storage ping succeeded", "component", "repository", "repository", "storage", "method", "Ping", "external_system", "minio", "bucket", s.minio.BucketName)
	return nil
}

type storageError string

func (e storageError) Error() string {
	return string(e)
}

const errStorageNotConfigured storageError = "storage is not configured"

func (s *StorageRepo) UploadFile(r io.Reader, objectName string, contentType string) (string, error) {
	ctx := context.Background()
	logger.Info("storage upload started", "component", "repository", "repository", "storage", "method", "UploadFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName, "content_type", contentType)

	_, err := s.minio.Client.PutObject(
		ctx,
		s.minio.BucketName,
		objectName,
		r,
		-1,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		logger.Error("storage upload failed", err, "component", "repository", "repository", "storage", "method", "UploadFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName, "content_type", contentType)
		return "", err
	}

	logger.Info("storage upload succeeded", "component", "repository", "repository", "storage", "method", "UploadFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName, "content_type", contentType)
	// Возвращаем внутренний путь (который храним в БД)
	return objectName, nil
}

func (s *StorageRepo) GetFile(objectName string) (io.ReadCloser, string, int64, error) {
	ctx := context.Background()
	logger.Info("storage download started", "component", "repository", "repository", "storage", "method", "GetFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName)

	obj, err := s.minio.Client.GetObject(
		ctx,
		s.minio.BucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		logger.Error("storage download failed", err, "component", "repository", "repository", "storage", "method", "GetFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName)
		return nil, "", 0, err
	}

	// Проверяем, что файл существует
	info, err := obj.Stat()
	if err != nil {
		obj.Close() // обязательно закрываем
		logger.Error("storage download stat failed", err, "component", "repository", "repository", "storage", "method", "GetFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName)
		return nil, "", 0, err
	}

	// Content-Type у MinIO лежит в info.ContentType
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	logger.Info("storage download succeeded", "component", "repository", "repository", "storage", "method", "GetFile", "external_system", "minio", "bucket", s.minio.BucketName, "object_name", objectName, "content_type", contentType, "size", info.Size)
	return obj, contentType, info.Size, nil
}

// GetPresignedURL — если тебе нужно временную ссылку
func (s *StorageRepo) GetPresignedURL(objectName string, duration time.Duration) (string, error) {
	ctx := context.Background()

	url, err := s.minio.Client.PresignedGetObject(
		ctx,
		s.minio.BucketName,
		objectName,
		duration,
		nil,
	)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

func (s *StorageRepo) DeleteFile(objectName string) error {
	ctx := context.Background()

	return s.minio.Client.RemoveObject(
		ctx,
		s.minio.BucketName,
		objectName,
		minio.RemoveObjectOptions{},
	)
}

// Exists — проверка существования объекта
func (s *StorageRepo) Exists(objectName string) bool {
	ctx := context.Background()

	_, err := s.minio.Client.StatObject(
		ctx,
		s.minio.BucketName,
		objectName,
		minio.StatObjectOptions{},
	)

	return err == nil
}

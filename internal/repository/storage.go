package repository

import (
	"context"
	"io"
	"time"
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
func (s *StorageRepo) UploadFile(r io.Reader, objectName string, contentType string) (string, error) {
	ctx := context.Background()

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
		return "", err
	}

	// Возвращаем внутренний путь (который храним в БД)
	return objectName, nil
}

func (s *StorageRepo) GetFile(objectName string) (io.ReadCloser, string, int64, error) {
	ctx := context.Background()

	obj, err := s.minio.Client.GetObject(
		ctx,
		s.minio.BucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, "", 0, err
	}

	// Проверяем, что файл существует
	info, err := obj.Stat()
	if err != nil {
		obj.Close() // обязательно закрываем
		return nil, "", 0, err
	}

	// Content-Type у MinIO лежит в info.ContentType
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

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

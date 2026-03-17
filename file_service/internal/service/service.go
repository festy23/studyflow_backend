package service

import (
	"common_library/ctxdata"
	"common_library/logging"
	"context"
	"errors"
	"fileservice/internal/errdefs"
	"fileservice/internal/model"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// privilegedRoles can access any file regardless of UploadedBy.
// Cross-user access for ordinary tutor/student flows must go through
// services (homework_service, payment_service) which re-issue the
// file_id under their own authorization checks.
var privilegedRoles = map[string]bool{
	"service": true,
	"admin":   true,
}

func authorizeFileAccess(ctx context.Context, file *model.File) error {
	if role, ok := ctxdata.GetUserRole(ctx); ok && privilegedRoles[role] {
		return nil
	}
	callerID, ok := ctxdata.GetUserID(ctx)
	if !ok || callerID == "" {
		return fmt.Errorf("missing caller identity: %w", errdefs.ErrPermissionDenied)
	}
	if callerID != file.UploadedBy.String() {
		return fmt.Errorf("caller is not the owner: %w", errdefs.ErrPermissionDenied)
	}
	return nil
}

type FileRepository interface {
	CreateFile(ctx context.Context, input *model.RepositoryCreateFileInput) (*model.File, error)
	GetFile(ctx context.Context, fileId uuid.UUID) (*model.File, error)
	ConfirmUpload(ctx context.Context, fileId uuid.UUID) (*model.File, error)
	ListOrphanUploads(ctx context.Context, olderThan time.Time) ([]*model.File, error)
	DeleteFile(ctx context.Context, fileId uuid.UUID) error
}

type FileService struct {
	fileRepo         FileRepository
	s3Client         *s3.Client
	bucket           *string
	gatewayPublicUrl string
	minioURL         string
}

func NewFileService(ctx context.Context, fileRepo FileRepository, client *s3.Client, bucketName string, gatewayPublicUrl string, minioUrl string) (*FileService, error) {
	s := &FileService{fileRepo: fileRepo, s3Client: client, bucket: aws.String(bucketName), gatewayPublicUrl: gatewayPublicUrl, minioURL: minioUrl}
	err := s.createBucket(ctx, bucketName)
	return s, err
}

var allowedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true, ".csv": true,
	".mp3": true, ".mp4": true, ".wav": true, ".zip": true, ".rar": true,
}

func (s *FileService) InitUpload(ctx context.Context, input *model.InitUploadInput) (*model.InitUpload, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	extension := strings.ToLower(path.Ext(input.Filename))
	if extension == "" {
		return nil, fmt.Errorf("invalid file extension: %w", errdefs.ErrValidation)
	}
	if !allowedExtensions[extension] {
		return nil, fmt.Errorf("file extension %s not allowed: %w", extension, errdefs.ErrValidation)
	}
	fileInput := &model.RepositoryCreateFileInput{
		Id:         id,
		Extension:  extension,
		UploadedBy: input.UploadedBy,
		Filename:   &input.Filename,
	}

	file, err := s.fileRepo.CreateFile(ctx, fileInput)
	if err != nil {
		return nil, err
	}

	key := file.Id.String() + file.Extension
	uploadRequest, err := s.generateUploadURL(ctx, key)
	if err != nil {
		return nil, err
	}

	rewritten, err := s.rewritePresignedURL(ctx, uploadRequest.URL, "/files/upload")
	if err != nil {
		return nil, err
	}

	res := &model.InitUpload{
		FileId:    file.Id,
		UploadURL: rewritten,
		Method:    uploadRequest.Method,
	}

	return res, nil
}

// rewritePresignedURL replaces the internal storage host with the
// public gateway host. Returns an internal error (without leaking the
// raw URL) if the presigned URL host does not match the configured
// MinIO host.
func (s *FileService) rewritePresignedURL(ctx context.Context, presigned, prefixPath string) (string, error) {
	presignedURL, err := url.Parse(presigned)
	if err != nil {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "failed to parse presigned URL", zap.Error(err))
		}
		return "", fmt.Errorf("could not rewrite upload URL: %w", errdefs.ErrInternal)
	}
	minioURL, err := url.Parse(s.minioURL)
	if err != nil {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "failed to parse minio URL", zap.Error(err))
		}
		return "", fmt.Errorf("could not rewrite upload URL: %w", errdefs.ErrInternal)
	}
	gatewayURL, err := url.Parse(s.gatewayPublicUrl)
	if err != nil {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "failed to parse gateway URL", zap.Error(err))
		}
		return "", fmt.Errorf("could not rewrite upload URL: %w", errdefs.ErrInternal)
	}

	if presignedURL.Host != minioURL.Host || presignedURL.Scheme != minioURL.Scheme {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "presigned URL host does not match configured storage host",
				zap.String("presigned_host", presignedURL.Host),
				zap.String("expected_host", minioURL.Host))
		}
		return "", fmt.Errorf("could not rewrite upload URL: %w", errdefs.ErrInternal)
	}

	presignedURL.Scheme = gatewayURL.Scheme
	presignedURL.Host = gatewayURL.Host
	presignedURL.Path = strings.TrimRight(gatewayURL.Path, "/") + prefixPath + presignedURL.Path
	return presignedURL.String(), nil
}

func (s *FileService) GenerateDownloadURL(ctx context.Context, fileId uuid.UUID) (string, error) {
	file, err := s.fileRepo.GetFile(ctx, fileId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("file not found: %w", errdefs.ErrNotFound)
		}
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	if err := authorizeFileAccess(ctx, file); err != nil {
		return "", err
	}
	if !file.IsUploaded {
		return "", fmt.Errorf("file upload is not confirmed: %w", errdefs.ErrNotFound)
	}

	key := file.Id.String() + file.Extension
	downloadRequest, err := s.generateDownloadURL(ctx, key)
	if err != nil {
		return "", err
	}

	return s.rewritePresignedURL(ctx, downloadRequest.URL, "/files/download")
}

func (s *FileService) GetFileMeta(ctx context.Context, fileId uuid.UUID) (*model.File, error) {
	file, err := s.fileRepo.GetFile(ctx, fileId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("file not found: %w", errdefs.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if err := authorizeFileAccess(ctx, file); err != nil {
		return nil, err
	}

	return file, nil
}

func (s *FileService) ConfirmUpload(ctx context.Context, fileId uuid.UUID) (*model.File, error) {
	file, err := s.fileRepo.GetFile(ctx, fileId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("file not found: %w", errdefs.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if err := authorizeFileAccess(ctx, file); err != nil {
		return nil, err
	}
	file, err = s.fileRepo.ConfirmUpload(ctx, fileId)
	if err != nil {
		return nil, fmt.Errorf("failed to confirm upload: %w", err)
	}
	return file, nil
}

func (s *FileService) CleanupOrphanUploads(ctx context.Context, olderThan time.Time) (int, error) {
	files, err := s.fileRepo.ListOrphanUploads(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to list orphan uploads: %w", err)
	}
	deleted := 0
	for _, file := range files {
		key := file.Id.String() + file.Extension
		if _, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: s.bucket,
			Key:    aws.String(key),
		}); err != nil {
			return deleted, fmt.Errorf("failed to delete orphan object: %w", err)
		}
		if err := s.fileRepo.DeleteFile(ctx, file.Id); err != nil {
			return deleted, fmt.Errorf("failed to delete orphan metadata: %w", err)
		}
		deleted++
	}
	return deleted, nil
}

func (s *FileService) createBucket(ctx context.Context, name string) error {
	_, err := s.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)})
	if err != nil {
		var opErr *http.ResponseError
		if errors.As(err, &opErr) && opErr.HTTPStatusCode() == 409 {
			if logger, ok := logging.GetFromContext(ctx); ok {
				logger.Info(ctx, "Bucket already exists", zap.String("bucket", name))
			}
			return nil
		}
	}
	return err
}

func (s *FileService) generateUploadURL(ctx context.Context, key string) (*v4.PresignedHTTPRequest, error) {
	presigner := s3.NewPresignClient(s.s3Client)

	req, err := presigner.PresignPutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket: s.bucket,
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(5*time.Minute),
	)
	return req, err
}

func (s *FileService) generateDownloadURL(ctx context.Context, key string) (*v4.PresignedHTTPRequest, error) {
	presigner := s3.NewPresignClient(s.s3Client)

	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: s.bucket,
		Key:    aws.String(key),
	},
		s3.WithPresignExpires(5*time.Minute),
	)

	return req, err
}

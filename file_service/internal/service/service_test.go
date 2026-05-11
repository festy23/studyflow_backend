package service

import (
	"common_library/ctxdata"
	"context"
	"errors"
	"fileservice/internal/errdefs"
	"fileservice/internal/model"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func ctxWithUser(userID uuid.UUID) context.Context {
	return ctxdata.WithUserID(context.Background(), userID.String())
}

// MockFileRepository is a testify mock for FileRepository.
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) CreateFile(ctx context.Context, input *model.RepositoryCreateFileInput) (*model.File, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.File), args.Error(1)
}

func (m *MockFileRepository) GetFile(ctx context.Context, fileId uuid.UUID) (*model.File, error) {
	args := m.Called(ctx, fileId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.File), args.Error(1)
}

func (m *MockFileRepository) ConfirmUpload(ctx context.Context, fileId uuid.UUID) (*model.File, error) {
	args := m.Called(ctx, fileId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.File), args.Error(1)
}

func (m *MockFileRepository) ListOrphanUploads(ctx context.Context, olderThan time.Time) ([]*model.File, error) {
	args := m.Called(ctx, olderThan)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.File), args.Error(1)
}

func (m *MockFileRepository) DeleteFile(ctx context.Context, fileId uuid.UUID) error {
	args := m.Called(ctx, fileId)
	return args.Error(0)
}

func newTestService(repo FileRepository) *FileService {
	return &FileService{
		fileRepo:         repo,
		bucket:           aws.String("test-bucket"),
		gatewayPublicUrl: "http://gateway",
		minioURL:         "http://minio:9000",
	}
}

func TestInitUpload_NoExtension(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	input := &model.InitUploadInput{
		UploadedBy: uuid.New(),
		Filename:   "testfile",
	}

	result, err := svc.InitUpload(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrValidation))
	mockRepo.AssertNotCalled(t, "CreateFile", mock.Anything, mock.Anything)
}

func TestInitUpload_DisallowedExtension(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	input := &model.InitUploadInput{
		UploadedBy: uuid.New(),
		Filename:   "test.exe",
	}

	result, err := svc.InitUpload(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrValidation))
	mockRepo.AssertNotCalled(t, "CreateFile", mock.Anything, mock.Anything)
}

func TestGetFileMeta_Success(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "document.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	result, err := svc.GetFileMeta(ctxWithUser(owner), fileId)

	assert.NoError(t, err)
	assert.Equal(t, expectedFile, result)
	mockRepo.AssertExpectations(t)
}

func TestGetFileMeta_PermissionDenied(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	other := uuid.New()
	filename := "document.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	result, err := svc.GetFileMeta(ctxWithUser(other), fileId)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	mockRepo.AssertExpectations(t)
}

func TestGenerateDownloadURL_PermissionDenied(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	other := uuid.New()
	filename := "doc.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	result, err := svc.GenerateDownloadURL(ctxWithUser(other), fileId)

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	mockRepo.AssertExpectations(t)
}

func TestGenerateDownloadURL_NoCallerIdentity(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "doc.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	result, err := svc.GenerateDownloadURL(context.Background(), fileId)

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	mockRepo.AssertExpectations(t)
}

func TestGetFileMeta_PrivilegedRole(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "doc.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	ctx := ctxdata.WithUserRole(context.Background(), "service")
	result, err := svc.GetFileMeta(ctx, fileId)

	assert.NoError(t, err)
	assert.Equal(t, expectedFile, result)
	mockRepo.AssertExpectations(t)
}

func TestGetFileMeta_NotFound(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	mockRepo.On("GetFile", mock.Anything, fileId).Return(nil, pgx.ErrNoRows)

	result, err := svc.GetFileMeta(context.Background(), fileId)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrNotFound))
	mockRepo.AssertExpectations(t)
}

func TestGenerateDownloadURL_NotFound(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	mockRepo.On("GetFile", mock.Anything, fileId).Return(nil, pgx.ErrNoRows)

	result, err := svc.GenerateDownloadURL(context.Background(), fileId)

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrNotFound))
	mockRepo.AssertExpectations(t)
}

func TestGenerateDownloadURL_NotConfirmed(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "doc.pdf"
	f := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		IsUploaded: false,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(f, nil)

	result, err := svc.GenerateDownloadURL(ctxWithUser(owner), fileId)

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrNotFound))
	mockRepo.AssertExpectations(t)
}

// --- ConfirmUpload tests ---

func TestConfirmUpload_Success(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "doc.pdf"
	f := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}
	confirmed := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		IsUploaded: true,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(f, nil)
	mockRepo.On("ConfirmUpload", mock.Anything, fileId).Return(confirmed, nil)

	result, err := svc.ConfirmUpload(ctxWithUser(owner), fileId)

	assert.NoError(t, err)
	assert.Equal(t, confirmed, result)
	assert.True(t, result.IsUploaded)
	mockRepo.AssertExpectations(t)
}

func TestConfirmUpload_PermissionDenied(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	other := uuid.New()
	filename := "doc.pdf"
	f := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(f, nil)

	result, err := svc.ConfirmUpload(ctxWithUser(other), fileId)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	mockRepo.AssertNotCalled(t, "ConfirmUpload", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
}

func TestConfirmUpload_NotFound(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	mockRepo.On("GetFile", mock.Anything, fileId).Return(nil, pgx.ErrNoRows)

	result, err := svc.ConfirmUpload(context.Background(), fileId)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrNotFound))
	mockRepo.AssertNotCalled(t, "ConfirmUpload", mock.Anything, mock.Anything)
	mockRepo.AssertExpectations(t)
}

func TestConfirmUpload_PrivilegedRole(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	owner := uuid.New()
	filename := "doc.pdf"
	f := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}
	confirmed := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: owner,
		Filename:   &filename,
		IsUploaded: true,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(f, nil)
	mockRepo.On("ConfirmUpload", mock.Anything, fileId).Return(confirmed, nil)

	ctx := ctxdata.WithUserRole(context.Background(), "service")
	result, err := svc.ConfirmUpload(ctx, fileId)

	assert.NoError(t, err)
	assert.Equal(t, confirmed, result)
	mockRepo.AssertExpectations(t)
}

// --- rewritePresignedURL tests ---

func TestRewritePresignedURL_Success(t *testing.T) {
	svc := newTestService(new(MockFileRepository))

	presigned := "http://minio:9000/test-bucket/abc123.pdf?signature=xyz"
	result, err := svc.rewritePresignedURL(context.Background(), presigned, "/files/")

	assert.NoError(t, err)
	assert.Contains(t, result, "gateway")
	assert.Contains(t, result, "/files/")
}

func TestRewritePresignedURL_HostMismatch(t *testing.T) {
	svc := newTestService(new(MockFileRepository))

	presigned := "http://other-host:9000/test-bucket/abc123.pdf"
	result, err := svc.rewritePresignedURL(context.Background(), presigned, "/files/")

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrInternal))
}

func TestRewritePresignedURL_InvalidPresignedURL(t *testing.T) {
	svc := newTestService(new(MockFileRepository))

	result, err := svc.rewritePresignedURL(context.Background(), "://invalid-url", "/files/")

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrInternal))
}

func TestRewritePresignedURL_InvalidMinioURL(t *testing.T) {
	svc := &FileService{
		fileRepo:         new(MockFileRepository),
		bucket:           aws.String("test-bucket"),
		gatewayPublicUrl: "http://gateway",
		minioURL:         "://invalid",
	}

	result, err := svc.rewritePresignedURL(context.Background(), "http://minio:9000/bucket/file.pdf", "/files/")

	assert.Empty(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errdefs.ErrInternal))
}

// --- CleanupOrphanUploads tests ---

func TestCleanupOrphanUploads_NoOrphans(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	mockRepo.On("ListOrphanUploads", mock.Anything, mock.Anything).Return([]*model.File{}, nil)

	count, err := svc.CleanupOrphanUploads(context.Background(), time.Now())

	assert.NoError(t, err)
	assert.Equal(t, 0, count)
	mockRepo.AssertExpectations(t)
}

func TestCleanupOrphanUploads_ListError(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	mockRepo.On("ListOrphanUploads", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

	count, err := svc.CleanupOrphanUploads(context.Background(), time.Now())

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	mockRepo.AssertExpectations(t)
}

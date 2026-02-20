package service

import (
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
	assert.True(t, errors.Is(err, errdefs.ValidationErr))
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
	assert.True(t, errors.Is(err, errdefs.ValidationErr))
	mockRepo.AssertNotCalled(t, "CreateFile", mock.Anything, mock.Anything)
}

func TestGetFileMeta_Success(t *testing.T) {
	mockRepo := new(MockFileRepository)
	svc := newTestService(mockRepo)

	fileId := uuid.New()
	filename := "document.pdf"
	expectedFile := &model.File{
		Id:         fileId,
		Extension:  ".pdf",
		UploadedBy: uuid.New(),
		Filename:   &filename,
		CreatedAt:  time.Now(),
	}

	mockRepo.On("GetFile", mock.Anything, fileId).Return(expectedFile, nil)

	result, err := svc.GetFileMeta(context.Background(), fileId)

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

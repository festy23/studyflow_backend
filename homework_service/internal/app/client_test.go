package app_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"homework_service/internal/app"
)

type IntegrationTestSuite struct {
	suite.Suite
	fileClient *app.FileClient
	userClient *app.UserClient
	conn       *grpc.ClientConn
}

func (s *IntegrationTestSuite) SetupSuite() {
	// Инициализация соединения
	var err error

	fileConn, err := grpc.NewClient(
		"localhost:50051", // адрес FileService
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		s.T().Fatalf("failed to connect to FileService: %v", err)
	}

	userConn, err := grpc.NewClient(
		"localhost:50052", // адрес UserService
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		s.T().Fatalf("failed to connect to UserService: %v", err)
	}

	s.conn = fileConn
	s.fileClient = app.NewFileClient(fileConn)
	s.userClient = app.NewUserClient(userConn)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *IntegrationTestSuite) TestFileClientIntegration() {
	t := s.T()

	fileID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-e3f4a5b6c7d8") // нужен реальный ID

	ctx := context.Background()
	url, err := s.fileClient.GetFileURL(ctx, fileID)

	assert.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "http") // проверка формата URL
}

func (s *IntegrationTestSuite) TestUserClientIntegration() {
	t := s.T()

	tutorID := uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")   // нужен реальный ID
	studentID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000") // нужен реальный ID

	ctx := context.Background()

	isPair, err := s.userClient.IsPair(ctx, tutorID, studentID)

	assert.NoError(t, err)
	assert.True(t, isPair)

	randomTutorID := uuid.New()
	randomStudentID := uuid.New()
	isPair, err = s.userClient.IsPair(ctx, randomTutorID, randomStudentID)

	assert.NoError(t, err)
	assert.False(t, isPair)
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	suite.Run(t, new(IntegrationTestSuite))
}

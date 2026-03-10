package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	userpb "userservice/pkg/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// chatIDResolver resolves a StudyFlow user_id to a Telegram chat_id.
type chatIDResolver interface {
	GetChatID(ctx context.Context, userID string) (int64, error)
}

// errChatIDNotFound means the user has no linked Telegram account — terminal,
// do not retry.
var errChatIDNotFound = errors.New("telegram chat_id not found for user")

type userServiceResolver struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
}

func dialUserService(addr string) (*userServiceResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user_service: %w", err)
	}
	return &userServiceResolver{
		conn:   conn,
		client: userpb.NewUserServiceClient(conn),
	}, nil
}

func (r *userServiceResolver) Close() {
	_ = r.conn.Close()
}

func (r *userServiceResolver) GetChatID(ctx context.Context, userID string) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := r.client.GetTelegramChatId(callCtx, &userpb.GetTelegramChatIdRequest{UserId: userID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, errChatIDNotFound
		}
		return 0, err
	}
	return resp.TelegramId, nil
}

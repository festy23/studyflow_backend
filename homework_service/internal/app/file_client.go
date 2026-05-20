package app

import (
	"common_library/ctxdata"
	"common_library/utils"
	"context"
	filePb "fileservice/pkg/api"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"homework_service/internal/service"
)

type FileClient struct {
	client filePb.FileServiceClient
}

func NewFileClient(conn *grpc.ClientConn) *FileClient {
	return &FileClient{client: filePb.NewFileServiceClient(conn)}
}

func (c *FileClient) outgoing(ctx context.Context) context.Context {
	out := metadata.NewOutgoingContext(ctx, metadata.Pairs())
	if id, ok := ctxdata.GetUserID(ctx); ok {
		out = metadata.AppendToOutgoingContext(out, "x-user-id", id)
	}
	out = metadata.AppendToOutgoingContext(out, "x-user-role", "service")
	return out
}

func (c *FileClient) GetFileURL(ctx context.Context, fileID uuid.UUID) (string, error) {
	outCtx := c.outgoing(ctx)
	resp, err := utils.RetryWithBackoff(outCtx, 3, 100*time.Millisecond, func() (*filePb.DownloadURL, error) {
		return c.client.GenerateDownloadURL(outCtx, &filePb.GenerateDownloadURLRequest{FileId: fileID.String()})
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// EnsureFileExists verifies that the given file_id refers to a file the caller
// can access in file_service. NotFound and PermissionDenied responses are
// translated to service.ErrInvalidFileID so that the gRPC layer surfaces
// codes.InvalidArgument to the client.
func (c *FileClient) EnsureFileExists(ctx context.Context, fileID uuid.UUID) error {
	outCtx := c.outgoing(ctx)
	_, err := utils.RetryWithBackoff(outCtx, 3, 100*time.Millisecond, func() (*filePb.File, error) {
		return c.client.GetFileMeta(outCtx, &filePb.GetFileMetaRequest{FileId: fileID.String()})
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound, codes.PermissionDenied, codes.InvalidArgument:
				return service.ErrInvalidFileID
			}
		}
		return fmt.Errorf("file_service GetFileMeta: %w", err)
	}
	return nil
}

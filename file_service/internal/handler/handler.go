package handler

import (
	"common_library/ctxdata"
	"common_library/logging"
	"context"
	"errors"
	"fileservice/internal/errdefs"
	"fileservice/internal/model"
	pb "fileservice/pkg/api"
	"slices"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FileService interface {
	InitUpload(ctx context.Context, input *model.InitUploadInput) (*model.InitUpload, error)
	GenerateDownloadURL(ctx context.Context, fileId uuid.UUID) (string, error)
	GetFileMeta(ctx context.Context, fileId uuid.UUID) (*model.File, error)
	ConfirmUpload(ctx context.Context, fileId uuid.UUID) (*model.File, error)
	CleanupOrphanUploads(ctx context.Context, olderThan time.Time) (int, error)
}

type FileHandler struct {
	pb.UnimplementedFileServiceServer
	fileService FileService
}

func NewFileHandler(fileService FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) InitUpload(ctx context.Context, req *pb.InitUploadRequest) (*pb.InitUploadResponse, error) {
	userId, err := uuid.Parse(req.UploadedBy)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input := &model.InitUploadInput{
		UploadedBy: userId,
		Filename:   req.Filename,
	}

	resp, err := h.fileService.InitUpload(ctx, input)
	if err != nil {
		return nil, mapError(ctx, err, errdefs.ErrValidation)
	}

	return toPbInitUpload(resp), nil
}

func (h *FileHandler) GenerateDownloadURL(ctx context.Context, req *pb.GenerateDownloadURLRequest) (*pb.DownloadURL, error) {
	id, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	resp, err := h.fileService.GenerateDownloadURL(ctx, id)
	if err != nil {
		return nil, mapError(ctx, err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	return &pb.DownloadURL{Url: resp}, nil
}

func (h *FileHandler) GetFileMeta(ctx context.Context, req *pb.GetFileMetaRequest) (*pb.File, error) {
	id, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	resp, err := h.fileService.GetFileMeta(ctx, id)
	if err != nil {
		return nil, mapError(ctx, err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	return toPbFile(resp), nil
}

func (h *FileHandler) ConfirmUpload(ctx context.Context, req *pb.ConfirmUploadRequest) (*pb.File, error) {
	id, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	resp, err := h.fileService.ConfirmUpload(ctx, id)
	if err != nil {
		return nil, mapError(ctx, err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}
	return toPbFile(resp), nil
}

func (h *FileHandler) CleanupOrphanUploads(ctx context.Context, req *pb.CleanupOrphanUploadsRequest) (*pb.CleanupOrphanUploadsResponse, error) {
	role, ok := ctxdata.GetUserRole(ctx)
	if !ok || role != "service" {
		return nil, status.Errorf(codes.PermissionDenied, "%v", errdefs.ErrPermissionDenied)
	}
	if req.OlderThan == nil {
		return nil, status.Error(codes.InvalidArgument, "older_than is required")
	}
	deleted, err := h.fileService.CleanupOrphanUploads(ctx, req.OlderThan.AsTime())
	if err != nil {
		return nil, mapError(ctx, err)
	}
	return &pb.CleanupOrphanUploadsResponse{DeletedCount: int32(deleted)}, nil //nolint:gosec // CleanupOrphanUploads returns count that fits in int32
}

func toPbInitUpload(init *model.InitUpload) *pb.InitUploadResponse {
	return &pb.InitUploadResponse{
		FileId:    init.FileId.String(),
		UploadUrl: init.UploadURL,
		Method:    init.Method,
	}
}

func toPbFile(file *model.File) *pb.File {
	return &pb.File{
		Id:         file.Id.String(),
		Extension:  file.Extension,
		UploadedBy: file.UploadedBy.String(),
		Filename:   file.Filename,
		CreatedAt:  timestamppb.New(file.CreatedAt),
		IsUploaded: file.IsUploaded,
	}
}

func mapError(ctx context.Context, err error, possibleErrors ...error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, errdefs.ErrAlreadyExists) && slices.Contains(possibleErrors, errdefs.ErrAlreadyExists):
		return status.Errorf(codes.AlreadyExists, "%v", err)

	case errors.Is(err, errdefs.ErrValidation) && slices.Contains(possibleErrors, errdefs.ErrValidation):
		return status.Errorf(codes.InvalidArgument, "%v", err)

	case errors.Is(err, errdefs.ErrAuthentication) && slices.Contains(possibleErrors, errdefs.ErrAuthentication):
		return status.Errorf(codes.Unauthenticated, "%v", err)

	case errors.Is(err, errdefs.ErrNotFound) && slices.Contains(possibleErrors, errdefs.ErrNotFound):
		return status.Errorf(codes.NotFound, "%v", err)

	case errors.Is(err, errdefs.ErrPermissionDenied) && slices.Contains(possibleErrors, errdefs.ErrPermissionDenied):
		return status.Errorf(codes.PermissionDenied, "%v", err)

	default:
		// Do not leak internal/AWS/S3 error strings to the client.
		// Log the underlying error for operators and return a
		// generic message on the wire.
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "internal error", zap.Error(err))
		}
		return status.Errorf(codes.Internal, "internal error")
	}
}

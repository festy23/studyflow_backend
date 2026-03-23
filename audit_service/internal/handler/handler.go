package handler

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"audit_service/internal/domain"
	"audit_service/internal/service"
	pb "audit_service/pkg/api"

	"common_library/logging"
)

// AuditService defines the interface the handler depends on.
type AuditService interface {
	RecordEvent(ctx context.Context, input *domain.RecordEventInput) (*domain.AuditEvent, error)
	ListEvents(ctx context.Context, input *domain.ListEventsInput) ([]*domain.AuditEvent, int64, error)
}

// AuditServiceServer implements the gRPC AuditServiceServer.
type AuditServiceServer struct {
	pb.UnimplementedAuditServiceServer
	svc AuditService
}

// NewAuditServiceServer creates a new AuditServiceServer.
func NewAuditServiceServer(svc AuditService) *AuditServiceServer {
	return &AuditServiceServer{svc: svc}
}

// RecordEvent handles the RecordEvent RPC.
func (h *AuditServiceServer) RecordEvent(ctx context.Context, req *pb.RecordEventRequest) (*pb.RecordEventResponse, error) {
	input := &domain.RecordEventInput{
		UserID:       req.UserId,
		Action:       req.Action,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceId,
		Details:      req.Details,
		IPAddress:    req.IpAddress,
	}

	if logger, ok := logging.GetFromContext(ctx); ok {
		logger.Info(ctx, "recording audit event",
			zap.String("user_id", input.UserID),
			zap.String("action", input.Action),
			zap.String("resource_type", input.ResourceType),
		)
	}

	event, err := h.svc.RecordEvent(ctx, input)
	if err != nil {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "failed to record audit event", zap.Error(err))
		}
		if errors.Is(err, service.ErrInvalidArgument) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.RecordEventResponse{
		Id: event.ID.String(),
	}, nil
}

// ListEvents handles the ListEvents RPC.
func (h *AuditServiceServer) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	input := &domain.ListEventsInput{
		UserID:       req.GetUserId(),
		ResourceType: req.GetResourceType(),
		ResourceID:   req.GetResourceId(),
	}
	if req.Page != nil {
		input.Page = *req.Page
	}
	if req.PageSize != nil {
		input.PageSize = *req.PageSize
	}

	if logger, ok := logging.GetFromContext(ctx); ok {
		logger.Info(ctx, "listing audit events",
			zap.String("user_id", input.UserID),
			zap.String("resource_type", input.ResourceType),
			zap.Int32("page", input.Page),
			zap.Int32("page_size", input.PageSize),
		)
	}

	events, totalCount, err := h.svc.ListEvents(ctx, input)
	if err != nil {
		if logger, ok := logging.GetFromContext(ctx); ok {
			logger.Error(ctx, "failed to list audit events", zap.Error(err))
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	pbEvents := make([]*pb.AuditEvent, 0, len(events))
	for _, e := range events {
		pbEvents = append(pbEvents, toPbEvent(e))
	}

	return &pb.ListEventsResponse{
		Events:     pbEvents,
		Page:       input.Page,
		TotalCount: totalCount,
	}, nil
}

func toPbEvent(e *domain.AuditEvent) *pb.AuditEvent {
	evt := &pb.AuditEvent{
		Id:           e.ID.String(),
		UserId:       e.UserID,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceId:   e.ResourceID,
		Details:      e.Details,
		IpAddress:    e.IPAddress,
		CreatedAt:    timestamppb.New(e.CreatedAt),
	}
	return evt
}

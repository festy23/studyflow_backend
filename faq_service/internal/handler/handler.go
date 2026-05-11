package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"faq_service/internal/domain"
	errdefs "faq_service/internal/errors"
	pb "faq_service/pkg/api"
)

type FAQServiceInterface interface {
	CreateFAQ(ctx context.Context, question, answer string, category *string, sortOrder int32) (*domain.FAQ, error)
	GetFAQ(ctx context.Context, id uuid.UUID) (*domain.FAQ, error)
	UpdateFAQ(ctx context.Context, id uuid.UUID, question, answer *string, category *string, sortOrder *int32) (*domain.FAQ, error)
	DeleteFAQ(ctx context.Context, id uuid.UUID) error
	ListFAQs(ctx context.Context, category *string, page, pageSize *int32) ([]*domain.FAQ, int64, error)
	ListCategories(ctx context.Context) ([]string, error)
}

type FAQServer struct {
	pb.UnimplementedFAQServiceServer
	svc FAQServiceInterface
}

func NewFAQServer(svc FAQServiceInterface) *FAQServer {
	return &FAQServer{svc: svc}
}

func (s *FAQServer) CreateFAQ(ctx context.Context, req *pb.CreateFAQRequest) (*pb.FAQ, error) {
	faq, err := s.svc.CreateFAQ(ctx, req.Question, req.Answer, req.Category, req.GetSortOrder())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoFAQ(faq), nil
}

func (s *FAQServer) GetFAQ(ctx context.Context, req *pb.GetFAQRequest) (*pb.FAQ, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	faq, err := s.svc.GetFAQ(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoFAQ(faq), nil
}

func (s *FAQServer) UpdateFAQ(ctx context.Context, req *pb.UpdateFAQRequest) (*pb.FAQ, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	faq, err := s.svc.UpdateFAQ(ctx, id, req.Question, req.Answer, req.Category, req.SortOrder)
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoFAQ(faq), nil
}

func (s *FAQServer) DeleteFAQ(ctx context.Context, req *pb.DeleteFAQRequest) (*pb.Empty, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.svc.DeleteFAQ(ctx, id); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

func (s *FAQServer) ListFAQs(ctx context.Context, req *pb.ListFAQsRequest) (*pb.ListFAQsResponse, error) {
	faqs, totalCount, err := s.svc.ListFAQs(ctx, req.Category, req.Page, req.PageSize)
	if err != nil {
		return nil, mapError(err)
	}
	pbFaqs := make([]*pb.FAQ, 0, len(faqs))
	for _, f := range faqs {
		pbFaqs = append(pbFaqs, toProtoFAQ(f))
	}
	page := int32(0)
	if req.Page != nil {
		page = *req.Page
	}
	return &pb.ListFAQsResponse{Faqs: pbFaqs, Page: page, TotalCount: totalCount}, nil
}

func (s *FAQServer) ListCategories(ctx context.Context, req *pb.Empty) (*pb.ListCategoriesResponse, error) {
	categories, err := s.svc.ListCategories(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.ListCategoriesResponse{Categories: categories}, nil
}

func toProtoFAQ(f *domain.FAQ) *pb.FAQ {
	return &pb.FAQ{
		Id:        f.ID.String(),
		Question:  f.Question,
		Answer:    f.Answer,
		Category:  f.Category,
		SortOrder: f.SortOrder,
		CreatedAt: timestamppb.New(f.CreatedAt),
		EditedAt:  timestamppb.New(f.EditedAt),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, errdefs.ErrNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, errdefs.ErrInvalidInput):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

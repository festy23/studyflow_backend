package service

import (
	"context"

	"github.com/google/uuid"

	"faq_service/internal/domain"
	errdefs "faq_service/internal/errors"
)

type IFAQRepo interface {
	Create(ctx context.Context, faq *domain.FAQ) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.FAQ, error)
	Update(ctx context.Context, faq *domain.FAQ) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, category *string, limit, offset int) ([]*domain.FAQ, int64, error)
	ListCategories(ctx context.Context) ([]string, error)
}

type FAQService struct {
	repo IFAQRepo
}

func NewFAQService(repo IFAQRepo) *FAQService {
	return &FAQService{repo: repo}
}

func (s *FAQService) CreateFAQ(ctx context.Context, question, answer string, category *string, sortOrder int32) (*domain.FAQ, error) {
	if question == "" || answer == "" {
		return nil, errdefs.ErrInvalidInput
	}
	faq := &domain.FAQ{
		Question:  question,
		Answer:    answer,
		Category:  category,
		SortOrder: sortOrder,
	}
	if err := s.repo.Create(ctx, faq); err != nil {
		return nil, err
	}
	return faq, nil
}

func (s *FAQService) GetFAQ(ctx context.Context, id uuid.UUID) (*domain.FAQ, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *FAQService) UpdateFAQ(ctx context.Context, id uuid.UUID, question, answer *string, category *string, sortOrder *int32) (*domain.FAQ, error) {
	faq, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if question != nil {
		faq.Question = *question
	}
	if answer != nil {
		faq.Answer = *answer
	}
	if category != nil {
		faq.Category = category
	}
	if sortOrder != nil {
		faq.SortOrder = *sortOrder
	}
	if err := s.repo.Update(ctx, faq); err != nil {
		return nil, err
	}
	return faq, nil
}

func (s *FAQService) DeleteFAQ(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *FAQService) ListFAQs(ctx context.Context, category *string, page, pageSize *int32) ([]*domain.FAQ, int64, error) {
	limit, offset := paginate(page, pageSize)
	return s.repo.List(ctx, category, limit, offset)
}

func (s *FAQService) ListCategories(ctx context.Context) ([]string, error) {
	return s.repo.ListCategories(ctx)
}

func paginate(page, pageSize *int32) (limit, offset int) {
	if page == nil {
		return 0, 0
	}
	p := int(*page)
	if p < 1 {
		p = 1
	}
	ps := 20
	if pageSize != nil && *pageSize > 0 {
		ps = int(*pageSize)
	}
	if ps > 100 {
		ps = 100
	}
	return ps, (p - 1) * ps
}

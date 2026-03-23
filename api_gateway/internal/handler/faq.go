package handler

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	faqpb "faq_service/pkg/api"
	"net/http"
	"time"
)

type FAQHandler struct {
	c     faqpb.FAQServiceClient
	cache Cache
}

func NewFAQHandler(c faqpb.FAQServiceClient, cache Cache) *FAQHandler {
	return &FAQHandler{c: c, cache: cache}
}

func (h *FAQHandler) RegisterRoutes(r chi.Router) {
	r.Get("/faqs", h.ListFAQs)
	r.Get("/faqs/categories", h.ListCategories)
	r.Get("/faqs/{id}", h.GetFAQ)
	r.Post("/faqs", h.CreateFAQ)
	r.Patch("/faqs/{id}", h.UpdateFAQ)
	r.Delete("/faqs/{id}", h.DeleteFAQ)
}

// ---------------------------------------------------------------------------
// Read handlers (cached)
// ---------------------------------------------------------------------------

func (h *FAQHandler) ListFAQs(w http.ResponseWriter, r *http.Request) {
	handler, err := HandleWithCache[faqpb.ListFAQsRequest, faqpb.ListFAQsResponse](
		h.c.ListFAQs, parseListFAQs, false,
		h.cache, buildFAQListKey, 5*time.Minute,
	)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *FAQHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	handler, err := HandleWithCache[faqpb.Empty, faqpb.ListCategoriesResponse](
		h.c.ListCategories, nil, false,
		h.cache, buildFAQCategoriesKey, 5*time.Minute,
	)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *FAQHandler) GetFAQ(w http.ResponseWriter, r *http.Request) {
	handler, err := HandleWithCache[faqpb.GetFAQRequest, faqpb.FAQ](
		h.c.GetFAQ, parseGetFAQ, false,
		h.cache, buildFAQItemKey, 5*time.Minute,
	)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

// ---------------------------------------------------------------------------
// Write handlers (no cache, invalidate on success)
// ---------------------------------------------------------------------------

func (h *FAQHandler) CreateFAQ(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[faqpb.CreateFAQRequest, faqpb.FAQ](h.c.CreateFAQ, nil, true)
	if err != nil {
		panic(err)
	}

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	handler(rec, r)

	if rec.status < http.StatusMultipleChoices {
		h.invalidateListCache(r.Context())
	}
}

func (h *FAQHandler) UpdateFAQ(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	handler, err := Handle[faqpb.UpdateFAQRequest, faqpb.FAQ](h.c.UpdateFAQ, parseUpdateFAQ, false)
	if err != nil {
		panic(err)
	}

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	handler(rec, r)

	if rec.status < http.StatusMultipleChoices {
		if id != "" {
			h.cache.Delete(r.Context(), fmt.Sprintf("faq:%s", id))
		}
		h.invalidateListCache(r.Context())
	}
}

func (h *FAQHandler) DeleteFAQ(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	handler, err := Handle[faqpb.DeleteFAQRequest, faqpb.Empty](h.c.DeleteFAQ, parseDeleteFAQ, false)
	if err != nil {
		panic(err)
	}

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	handler(rec, r)

	if rec.status < http.StatusMultipleChoices {
		if id != "" {
			h.cache.Delete(r.Context(), fmt.Sprintf("faq:%s", id))
		}
		h.invalidateListCache(r.Context())
	}
}

// ---------------------------------------------------------------------------
// Cache helpers
// ---------------------------------------------------------------------------

// invalidateListCache removes all faq:list:* entries and the categories
// cache entry using Redis SCAN-based pattern deletion.
func (h *FAQHandler) invalidateListCache(ctx context.Context) {
	h.cache.DeleteByPattern(ctx, "faq:list:*")
	h.cache.Delete(ctx, "faq:categories")
}

// ---------------------------------------------------------------------------
// Request parsers
// ---------------------------------------------------------------------------

func parseListFAQs(ctx context.Context, r *http.Request, req *faqpb.ListFAQsRequest) error {
	if category := r.URL.Query().Get("category"); category != "" {
		req.Category = &category
	}
	req.Page = parseInt32Ptr(r.URL.Query().Get("page"))
	req.PageSize = parseInt32Ptr(r.URL.Query().Get("page_size"))
	return nil
}

func parseGetFAQ(ctx context.Context, r *http.Request, req *faqpb.GetFAQRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.Id = id
	return nil
}

func parseUpdateFAQ(ctx context.Context, r *http.Request, req *faqpb.UpdateFAQRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.Id = id
	return nil
}

func parseDeleteFAQ(ctx context.Context, r *http.Request, req *faqpb.DeleteFAQRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.Id = id
	return nil
}

// ---------------------------------------------------------------------------
// Cache key builders
// ---------------------------------------------------------------------------

func buildFAQListKey(r *http.Request) (string, error) {
	category := r.URL.Query().Get("category")
	return fmt.Sprintf("faq:list:%s", category), nil
}

func buildFAQCategoriesKey(r *http.Request) (string, error) {
	return "faq:categories", nil
}

func buildFAQItemKey(r *http.Request) (string, error) {
	id := chi.URLParam(r, "id")
	if id == "" {
		return "", fmt.Errorf("missing path param: id")
	}
	return fmt.Sprintf("faq:%s", id), nil
}

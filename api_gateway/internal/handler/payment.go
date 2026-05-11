package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
	paymentpb "paymentservice/pkg/api"
)

type PaymentHandler struct {
	c paymentpb.PaymentServiceClient
}

func NewPaymentHandler(c paymentpb.PaymentServiceClient) *PaymentHandler {
	return &PaymentHandler{c: c}
}

func (h *PaymentHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.With(authMiddleware).Group(func(r chi.Router) {
		r.Get("/info/{lesson_id}", h.GetPaymentInfo)
		r.Get("/analytics/tutor/{tutor_id}", h.GetTutorAnalytics)
		r.Get("/receipts", h.ListReceipts)
		r.Post("/receipts", h.SubmitReceipt)
		r.Get("/receipts/{id}", h.GetReceipt)
		r.Post("/receipts/{id}/verify", h.VerifyReceipt)
		r.Get("/receipts/{id}/file-url", h.GetReceiptFile)
	})
}

func parseGetPaymentInfo(ctx context.Context, r *http.Request, req *paymentpb.GetPaymentInfoRequest) error {
	id, err := parsePathParam(r, "lesson_id")
	if err != nil {
		return err
	}
	req.LessonId = &id
	return nil
}

func parseGetReceipt(ctx context.Context, r *http.Request, req *paymentpb.GetReceiptRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.ReceiptId = id
	return nil
}

func parseVerifyReceipt(ctx context.Context, r *http.Request, req *paymentpb.VerifyReceiptRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.ReceiptId = id
	return nil
}

func parseListReceipts(ctx context.Context, r *http.Request, req *paymentpb.ListReceiptsRequest) error {
	if tutorID := r.URL.Query().Get("tutor_id"); tutorID != "" {
		req.TutorId = &tutorID
	}
	if studentID := r.URL.Query().Get("student_id"); studentID != "" {
		req.StudentId = &studentID
	}
	return nil
}

func parseGetReceiptFile(ctx context.Context, r *http.Request, req *paymentpb.GetReceiptFileRequest) error {
	id, err := parsePathParam(r, "id")
	if err != nil {
		return err
	}
	req.ReceiptId = id
	return nil
}

func parseGetTutorAnalytics(ctx context.Context, r *http.Request, req *paymentpb.GetTutorAnalyticsRequest) error {
	tutorID, err := parsePathParam(r, "tutor_id")
	if err != nil {
		return err
	}
	req.TutorId = tutorID

	query := r.URL.Query()
	if raw := query.Get("from"); raw != "" {
		from, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return ErrBadRequest
		}
		req.From = timestamppb.New(from)
	}
	if raw := query.Get("to"); raw != "" {
		to, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return ErrBadRequest
		}
		req.To = timestamppb.New(to)
	}
	return nil
}

func (h *PaymentHandler) GetPaymentInfo(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.GetPaymentInfoRequest, paymentpb.PaymentInfo](h.c.GetPaymentInfo, parseGetPaymentInfo, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) SubmitReceipt(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.SubmitPaymentReceiptRequest, paymentpb.Receipt](h.c.SubmitPaymentReceipt, nil, true)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) GetReceipt(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.GetReceiptRequest, paymentpb.Receipt](h.c.GetReceipt, parseGetReceipt, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) VerifyReceipt(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.VerifyReceiptRequest, paymentpb.Receipt](h.c.VerifyReceipt, parseVerifyReceipt, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) GetReceiptFile(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.GetReceiptFileRequest, paymentpb.ReceiptFileURL](h.c.GetReceiptFile, parseGetReceiptFile, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) ListReceipts(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.ListReceiptsRequest, paymentpb.ListReceiptsResponse](h.c.ListReceipts, parseListReceipts, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *PaymentHandler) GetTutorAnalytics(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[paymentpb.GetTutorAnalyticsRequest, paymentpb.TutorAnalytics](h.c.GetTutorAnalytics, parseGetTutorAnalytics, false)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

package handler

import (
	"common_library/logging"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	homeworkpb "homework_service/pkg/api"
)

// handlerInitError writes a 500 and logs when Handle returns nil + err.
func handlerInitError(w http.ResponseWriter, r *http.Request, err error) {
	if logger, ok := logging.GetFromContext(r.Context()); ok {
		logger.Error(r.Context(), "failed to construct handler", zap.Error(err))
	}
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

type contextKey string

type HomeworkHandler struct {
	c homeworkpb.HomeworkServiceClient
}

func NewHomeworkHandler(c homeworkpb.HomeworkServiceClient) *HomeworkHandler {
	return &HomeworkHandler{c: c}
}

func (h *HomeworkHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.With(authMiddleware).Group(func(r chi.Router) {
		r.Post("/assignments", h.CreateAssignment)
		r.Get("/assignments", h.ListAssignments)
		r.Patch("/assignments/{id}", h.UpdateAssignment)
		r.Delete("/assignments/{id}", h.DeleteAssignment)
		r.Get("/assignments/{assignment_id}/file-url", h.GetAssignmentFile)
		r.Get("/assignments/{assignment_id}/submissions", h.ListSubmissions)
		r.Get("/assignments/{assignment_id}/feedbacks", h.ListFeedbacks)

		r.Post("/submissions", h.CreateSubmission)
		r.Get("/submissions/{submission_id}/file-url", h.GetSubmissionFile)

		r.Post("/feedbacks", h.CreateFeedback)
		r.Patch("/feedbacks/{id}", h.UpdateFeedback)
		r.Get("/feedbacks/{feedback_id}/file-url", h.GetFeedbackFile)
	})
}

func parseAssignmentID(ctx context.Context, r *http.Request, req *homeworkpb.GetAssignmentFileRequest) error {
	id, err := parsePathParam(r, "assignment_id")
	if err != nil {
		return err
	}
	req.AssignmentId = id
	return nil
}

func parseSubmissionID(ctx context.Context, r *http.Request, req *homeworkpb.GetSubmissionFileRequest) error {
	id, err := parsePathParam(r, "submission_id")
	if err != nil {
		return err
	}
	req.SubmissionId = id
	return nil
}

func parseFeedbackID(ctx context.Context, r *http.Request, req *homeworkpb.GetFeedbackFileRequest) error {
	id, err := parsePathParam(r, "feedback_id")
	if err != nil {
		return err
	}
	req.FeedbackId = id
	return nil
}

func parseAssignmentQuery(ctx context.Context, r *http.Request) (context.Context, any, error) {
	q := r.URL.Query()
	tutorID := q.Get("tutor_id")
	studentID := q.Get("student_id")
	statuses := q["status_filter"]

	parseStatuses := func(raw []string) []homeworkpb.AssignmentStatusFilter {
		res := make([]homeworkpb.AssignmentStatusFilter, 0)
		for _, s := range raw {
			switch strings.ToUpper(s) {
			case "UNSENT":
				res = append(res, homeworkpb.AssignmentStatusFilter_UNSENT)
			case "UNREVIEWED":
				res = append(res, homeworkpb.AssignmentStatusFilter_UNREVIEWED)
			case "REVIEWED":
				res = append(res, homeworkpb.AssignmentStatusFilter_REVIEWED)
			case "OVERDUE":
				res = append(res, homeworkpb.AssignmentStatusFilter_OVERDUE)
			}
		}
		return res
	}

	page := parseInt32Ptr(q.Get("page"))
	pageSize := parseInt32Ptr(q.Get("page_size"))

	switch {
	case tutorID != "" && studentID != "":
		req := &homeworkpb.ListAssignmentsByPairRequest{TutorId: tutorID, StudentId: studentID, StatusFilter: parseStatuses(statuses), Page: page, PageSize: pageSize}
		return ctx, req, nil
	case tutorID != "":
		req := &homeworkpb.ListAssignmentsByTutorRequest{TutorId: tutorID, StatusFilter: parseStatuses(statuses), Page: page, PageSize: pageSize}
		return ctx, req, nil
	case studentID != "":
		req := &homeworkpb.ListAssignmentsByStudentRequest{StudentId: studentID, StatusFilter: parseStatuses(statuses), Page: page, PageSize: pageSize}
		return ctx, req, nil
	default:
		return nil, nil, fmt.Errorf("invalid filter combination")
	}
}

func (h *HomeworkHandler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.CreateAssignmentRequest, homeworkpb.Assignment](h.c.CreateAssignment, nil, true)
	if err != nil {
		panic(err)
	}
	handler(w, r)
}

func (h *HomeworkHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, req, err := parseAssignmentQuery(ctx, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch x := req.(type) {
	case *homeworkpb.ListAssignmentsByTutorRequest:
		handler, herr := Handle[homeworkpb.ListAssignmentsByTutorRequest, homeworkpb.ListAssignmentsResponse](
			h.c.ListAssignmentsByTutor,
			func(_ context.Context, _ *http.Request, grpcReq *homeworkpb.ListAssignmentsByTutorRequest) error {
				*grpcReq = *x
				return nil
			}, false)
		if herr != nil {
			handlerInitError(w, r, herr)
			return
		}
		handler(w, r)
	case *homeworkpb.ListAssignmentsByStudentRequest:
		handler, herr := Handle[homeworkpb.ListAssignmentsByStudentRequest, homeworkpb.ListAssignmentsResponse](
			h.c.ListAssignmentsByStudent,
			func(_ context.Context, _ *http.Request, grpcReq *homeworkpb.ListAssignmentsByStudentRequest) error {
				*grpcReq = *x
				return nil
			}, false)
		if herr != nil {
			handlerInitError(w, r, herr)
			return
		}
		handler(w, r)
	case *homeworkpb.ListAssignmentsByPairRequest:
		handler, herr := Handle[homeworkpb.ListAssignmentsByPairRequest, homeworkpb.ListAssignmentsResponse](
			h.c.ListAssignmentsByPair,
			func(_ context.Context, _ *http.Request, grpcReq *homeworkpb.ListAssignmentsByPairRequest) error {
				*grpcReq = *x
				return nil
			}, false)
		if herr != nil {
			handlerInitError(w, r, herr)
			return
		}
		handler(w, r)
	default:
		http.Error(w, "invalid query", http.StatusBadRequest)
	}
}

func (h *HomeworkHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.UpdateAssignmentRequest, homeworkpb.Assignment](h.c.UpdateAssignment, func(ctx context.Context, r *http.Request, req *homeworkpb.UpdateAssignmentRequest) error {
		id, err := parsePathParam(r, "id")
		if err != nil {
			return err
		}
		req.Id = id
		return nil
	}, true)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.DeleteAssignmentRequest, homeworkpb.Empty](h.c.DeleteAssignment, func(ctx context.Context, r *http.Request, req *homeworkpb.DeleteAssignmentRequest) error {
		id, err := parsePathParam(r, "id")
		if err != nil {
			return err
		}
		req.AssignmentId = id
		return nil
	}, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) GetAssignmentFile(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.GetAssignmentFileRequest, homeworkpb.HomeworkFileURL](h.c.GetAssignmentFile, parseAssignmentID, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.ListSubmissionsByAssignmentRequest, homeworkpb.ListSubmissionsResponse](h.c.ListSubmissionsByAssignment, func(ctx context.Context, r *http.Request, req *homeworkpb.ListSubmissionsByAssignmentRequest) error {
		id, err := parsePathParam(r, "assignment_id")
		if err != nil {
			return err
		}
		req.AssignmentId = id
		return nil
	}, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.CreateSubmissionRequest, homeworkpb.Submission](h.c.CreateSubmission, nil, true)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) GetSubmissionFile(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.GetSubmissionFileRequest, homeworkpb.HomeworkFileURL](h.c.GetSubmissionFile, parseSubmissionID, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) CreateFeedback(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.CreateFeedbackRequest, homeworkpb.Feedback](h.c.CreateFeedback, nil, true)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) UpdateFeedback(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.UpdateFeedbackRequest, homeworkpb.Feedback](h.c.UpdateFeedback, func(ctx context.Context, r *http.Request, req *homeworkpb.UpdateFeedbackRequest) error {
		id, err := parsePathParam(r, "id")
		if err != nil {
			return err
		}
		req.Id = id
		return nil
	}, true)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) ListFeedbacks(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.ListFeedbacksByAssignmentRequest, homeworkpb.ListFeedbacksResponse](h.c.ListFeedbacksByAssignment, func(ctx context.Context, r *http.Request, req *homeworkpb.ListFeedbacksByAssignmentRequest) error {
		id, err := parsePathParam(r, "assignment_id")
		if err != nil {
			return err
		}
		req.AssignmentId = id
		return nil
	}, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

func (h *HomeworkHandler) GetFeedbackFile(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[homeworkpb.GetFeedbackFileRequest, homeworkpb.HomeworkFileURL](h.c.GetFeedbackFile, parseFeedbackID, false)
	if err != nil {
		handlerInitError(w, r, err)
		return
	}
	handler(w, r)
}

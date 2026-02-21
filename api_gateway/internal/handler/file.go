package handler

import (
	"common_library/logging"
	"context"
	filepb "fileservice/pkg/api"
	"fmt"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type FileHandler struct {
	c              filepb.FileServiceClient
	minioUrl       string
	parsedMinioUrl *url.URL
	httpClient     *http.Client
}

func NewFileHandler(c filepb.FileServiceClient, minioUrl string) *FileHandler {
	parsed, err := url.Parse(minioUrl)
	if err != nil {
		panic(fmt.Sprintf("invalid minioUrl %q: %v", minioUrl, err))
	}
	return &FileHandler{
		c:              c,
		minioUrl:       minioUrl,
		parsedMinioUrl: parsed,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *FileHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.With(authMiddleware).Post("/init-upload", h.InitUpload)
	r.With(authMiddleware).Get("/{id}/meta", h.GetFileMeta)
	r.Put("/upload/*", h.proxyToMinio("PUT", "/files/upload"))
	r.Get("/download/*", h.proxyToMinio("GET", "/files/download"))
}

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[filepb.InitUploadRequest, filepb.InitUploadResponse](h.c.InitUpload, nil, true)
	if err != nil {
		panic(err)
	}

	handler(w, r)
}

func (h *FileHandler) GetFileMeta(w http.ResponseWriter, r *http.Request) {
	handler, err := Handle[filepb.GetFileMetaRequest, filepb.File](h.c.GetFileMeta, getFileMetaParsePath, false)
	if err != nil {
		panic(err)
	}

	handler(w, r)
}

func getFileMetaParsePath(ctx context.Context, httpReq *http.Request, grpcReq *filepb.GetFileMetaRequest) error {
	id := chi.URLParam(httpReq, "id")
	if id == "" {
		return fmt.Errorf("%w: %s", ErrBadRequest, "file id is required")
	}
	grpcReq.FileId = id

	if logger, ok := logging.GetFromContext(ctx); ok {
		logger.Debug(ctx, "file id added to request", zap.Any("req", grpcReq))
	}
	return nil
}

func (h *FileHandler) proxyToMinio(method string, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logger, ok := logging.GetFromContext(r.Context()); ok {
			logger.Debug(r.Context(), "request path", zap.String("path", r.URL.Path))
		}
		targetPath := strings.TrimPrefix(r.URL.Path, path)
		targetURL := h.minioUrl + targetPath + "?" + r.URL.RawQuery

		parsedURL, err := url.Parse(targetURL)
		if err != nil || parsedURL.Scheme != h.parsedMinioUrl.Scheme || parsedURL.Host != h.parsedMinioUrl.Host {
			http.Error(w, "Invalid proxy target", http.StatusBadRequest)
			return
		}

		req, err := http.NewRequest(method, targetURL, r.Body) //nolint:gosec // URL validated above (scheme+host check)
		if err != nil {
			http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		if clStr := r.Header.Get("Content-Length"); clStr != "" {
			if cl, err := strconv.ParseInt(clStr, 10, 64); err == nil {
				req.ContentLength = cl
			}
		}
		if logger, ok := logging.GetFromContext(r.Context()); ok {
			logger.Debug(r.Context(), "making proxy request", zap.String("URL", targetURL), zap.String("method", method), zap.Any("headers", req.Header))
		}
		resp, err := h.httpClient.Do(req) //nolint:gosec // URL validated above (scheme+host check)
		if err != nil {
			if logger, ok := logging.GetFromContext(r.Context()); ok {
				logger.Error(r.Context(), "Failed to proxy request", zap.Error(err))
			}
			http.Error(w, "Failed to reach MinIO", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			if logger, ok := logging.GetFromContext(r.Context()); ok {
				logger.Error(r.Context(), "failed to proxy response body", zap.Error(err))
			}
		}
	}
}

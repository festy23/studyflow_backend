FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct
COPY faq_service/go.mod faq_service/go.sum ./
COPY common_library/ /common_library/
RUN go mod download
COPY faq_service/ ./
RUN CGO_ENABLED=0 go build -o /faq-service ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /faq-service /faq-service
COPY --from=builder /app/migrations /migrations/
EXPOSE 50051
CMD ["/faq-service"]

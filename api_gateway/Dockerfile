FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct

COPY api_gateway/go.mod  ./
COPY api_gateway/go.sum  ./

COPY user_service/         /user_service/
COPY file_service/         /file_service/
COPY homework_service/     /homework_service/
COPY schedule_service/     /schedule_service/
COPY payment_service/      /payment_service/
COPY faq_service/          /faq_service/
COPY audit_service/        /audit_service/

COPY common_library/   /common_library/

RUN go mod download

COPY api_gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct
COPY --from=builder /server ./

EXPOSE 8080
CMD ["./server"]

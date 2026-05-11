FROM golang:1.25-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct
COPY audit_service/go.mod audit_service/go.sum ./
COPY common_library/ /common_library/
RUN go mod download

COPY audit_service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations/

EXPOSE 50051
CMD ["./server"]

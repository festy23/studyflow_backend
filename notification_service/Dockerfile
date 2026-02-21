FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY notification_service/go.mod  ./
COPY notification_service/go.sum  ./

RUN go mod download

COPY notification_service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/main.go

FROM alpine:latest
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /server ./
USER appuser

CMD ["./server"]

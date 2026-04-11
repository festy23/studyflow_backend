FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct

COPY notification_service/go.mod  ./
COPY notification_service/go.sum  ./

COPY user_service/ /user_service/

RUN go mod download

COPY notification_service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/

FROM alpine:latest
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
RUN apk add --no-cache git
ENV GOPROXY=https://goproxy.cn,direct
COPY --from=builder /server ./
USER appuser

CMD ["./server"]

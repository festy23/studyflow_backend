module notification_service

go 1.24.0

toolchain go1.24.2

require (
	github.com/segmentio/kafka-go v0.4.47
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.72.0
	userservice v0.0.0-00010101000000-000000000000
)

replace userservice => ../user_service

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250422160041-2d3770c4ea7f // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

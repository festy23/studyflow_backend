package api

import (
	context "context"
	grpc "google.golang.org/grpc"
)

// AuditServiceClient is the client API for AuditService service.
type AuditServiceClient interface {
	RecordEvent(ctx context.Context, in *RecordEventRequest, opts ...grpc.CallOption) (*RecordEventResponse, error)
	ListEvents(ctx context.Context, in *ListEventsRequest, opts ...grpc.CallOption) (*ListEventsResponse, error)
}

type auditServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuditServiceClient(cc grpc.ClientConnInterface) AuditServiceClient {
	return &auditServiceClient{cc}
}

func (c *auditServiceClient) RecordEvent(ctx context.Context, in *RecordEventRequest, opts ...grpc.CallOption) (*RecordEventResponse, error) {
	out := new(RecordEventResponse)
	err := c.cc.Invoke(ctx, "/audit.v1.AuditService/RecordEvent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *auditServiceClient) ListEvents(ctx context.Context, in *ListEventsRequest, opts ...grpc.CallOption) (*ListEventsResponse, error) {
	out := new(ListEventsResponse)
	err := c.cc.Invoke(ctx, "/audit.v1.AuditService/ListEvents", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

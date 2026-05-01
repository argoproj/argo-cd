package application

import (
	corev1 "k8s.io/api/core/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// eventListMessage wraps *corev1.EventList so it satisfies the proto.Message interface
// required by grpc-gateway v2 generated code at compile time.
// ProtoReflect is never called at runtime: HTTP responses are serialized via encoding/json.
type eventListMessage struct {
	*corev1.EventList
}

func (e *eventListMessage) ProtoReflect() protoreflect.Message { return nil }

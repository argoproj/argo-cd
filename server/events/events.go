package events

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// EventListToStruct converts a Kubernetes EventList to a protobuf Struct.
// This is used to return EventList data through gRPC APIs in a way that avoids
// protobuf compatibility issues with Kubernetes types in K8s 1.35+.
func EventListToStruct(eventList *corev1.EventList) (*structpb.Struct, error) {
	if eventList == nil {
		return structpb.NewStruct(map[string]interface{}{
			"metadata": map[string]interface{}{},
			"items":    []interface{}{},
		})
	}

	jsonBytes, err := json.Marshal(eventList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EventList to JSON: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EventList JSON: %w", err)
	}

	result, err := structpb.NewStruct(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create protobuf Struct: %w", err)
	}

	return result, nil
}

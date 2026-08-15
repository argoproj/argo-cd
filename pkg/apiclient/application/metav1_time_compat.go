package application

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// metav1TimeProtoShim adapts *metav1.Time to the legacy (github.com/golang/protobuf
// v1) message interface expected by google.golang.org/protobuf.
//
// k8s.io/apimachinery v0.36 removed the gogo ProtoMessage() marker method from
// its generated types, so *metav1.Time no longer satisfies either the v1 or v2
// proto.Message interface. The generated application.pb.go references
// metav1.Time in its dependency table (ApplicationPodLogsQuery.sinceTime and
// LogEntry.timeStamp), and google.golang.org/protobuf panics while resolving
// that dependency if the referenced Go type implements neither interface.
//
// hack/generate-proto.sh rewrites the goTypes entry for metav1.Time in
// application.pb.go to use this shim. Wire (de)serialization of the actual
// struct fields keeps using metav1.Time's own Marshal/Unmarshal methods via
// google.golang.org/protobuf's legacy support, so encoding is unchanged.
type metav1TimeProtoShim metav1.Time

func (m *metav1TimeProtoShim) Reset()         { (*metav1.Time)(m).Reset() }
func (m *metav1TimeProtoShim) String() string { return (*metav1.Time)(m).String() }
func (*metav1TimeProtoShim) ProtoMessage()    {}

func (m *metav1TimeProtoShim) Marshal() ([]byte, error) { return (*metav1.Time)(m).Marshal() }
func (m *metav1TimeProtoShim) Unmarshal(b []byte) error { return (*metav1.Time)(m).Unmarshal(b) }
func (m *metav1TimeProtoShim) Size() int                { return (*metav1.Time)(m).Size() }

package v1alpha1

// objectMeta and corresponding GetMetadata() methods is a hack to allow us to use grpc-gateway
// side-by-side with k8s protobuf codegen. The grpc-gateway generated .gw.pb.go files expect a
// GetMetadata() method to be generated because it assumes the .proto files were generated from
// protoc --go_out=plugins=grpc. Instead, kubernetes uses go-to-protobuf to generate .proto files
// from go types, and this method is not auto-generated (presumably since ObjectMeta is embedded but
// is nested in the 'metadata' field in JSON form).
type objectMeta struct {
	Name *string
}

func (a *Application) GetMetadata() *objectMeta {
	var om objectMeta
	if a != nil {
		om.Name = &a.Name
	}
	return &om
}

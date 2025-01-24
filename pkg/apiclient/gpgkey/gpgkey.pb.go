// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: server/gpgkey/gpgkey.proto

// GPG public key service
//
// GPG public key API performs CRUD actions against GnuPGPublicKey resources

package gpgkey

import (
	context "context"
	fmt "fmt"
	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	proto "github.com/gogo/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// Message to query the server for configured GPG public keys
type GnuPGPublicKeyQuery struct {
	// The GPG key ID to query for
	KeyID                string   `protobuf:"bytes,1,opt,name=keyID,proto3" json:"keyID,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GnuPGPublicKeyQuery) Reset()         { *m = GnuPGPublicKeyQuery{} }
func (m *GnuPGPublicKeyQuery) String() string { return proto.CompactTextString(m) }
func (*GnuPGPublicKeyQuery) ProtoMessage()    {}
func (*GnuPGPublicKeyQuery) Descriptor() ([]byte, []int) {
	return fileDescriptor_8ba55a5eb76dc6fd, []int{0}
}
func (m *GnuPGPublicKeyQuery) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GnuPGPublicKeyQuery) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GnuPGPublicKeyQuery.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GnuPGPublicKeyQuery) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GnuPGPublicKeyQuery.Merge(m, src)
}
func (m *GnuPGPublicKeyQuery) XXX_Size() int {
	return m.Size()
}
func (m *GnuPGPublicKeyQuery) XXX_DiscardUnknown() {
	xxx_messageInfo_GnuPGPublicKeyQuery.DiscardUnknown(m)
}

var xxx_messageInfo_GnuPGPublicKeyQuery proto.InternalMessageInfo

func (m *GnuPGPublicKeyQuery) GetKeyID() string {
	if m != nil {
		return m.KeyID
	}
	return ""
}

// Request to create one or more public keys on the server
type GnuPGPublicKeyCreateRequest struct {
	// Raw key data of the GPG key(s) to create
	Publickey *v1alpha1.GnuPGPublicKey `protobuf:"bytes,1,opt,name=publickey,proto3" json:"publickey,omitempty"`
	// Whether to upsert already existing public keys
	Upsert               bool     `protobuf:"varint,2,opt,name=upsert,proto3" json:"upsert,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GnuPGPublicKeyCreateRequest) Reset()         { *m = GnuPGPublicKeyCreateRequest{} }
func (m *GnuPGPublicKeyCreateRequest) String() string { return proto.CompactTextString(m) }
func (*GnuPGPublicKeyCreateRequest) ProtoMessage()    {}
func (*GnuPGPublicKeyCreateRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_8ba55a5eb76dc6fd, []int{1}
}
func (m *GnuPGPublicKeyCreateRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GnuPGPublicKeyCreateRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GnuPGPublicKeyCreateRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GnuPGPublicKeyCreateRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GnuPGPublicKeyCreateRequest.Merge(m, src)
}
func (m *GnuPGPublicKeyCreateRequest) XXX_Size() int {
	return m.Size()
}
func (m *GnuPGPublicKeyCreateRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GnuPGPublicKeyCreateRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GnuPGPublicKeyCreateRequest proto.InternalMessageInfo

func (m *GnuPGPublicKeyCreateRequest) GetPublickey() *v1alpha1.GnuPGPublicKey {
	if m != nil {
		return m.Publickey
	}
	return nil
}

func (m *GnuPGPublicKeyCreateRequest) GetUpsert() bool {
	if m != nil {
		return m.Upsert
	}
	return false
}

// Response to a public key creation request
type GnuPGPublicKeyCreateResponse struct {
	// List of GPG public keys that have been created
	Created *v1alpha1.GnuPGPublicKeyList `protobuf:"bytes,1,opt,name=created,proto3" json:"created,omitempty"`
	// List of key IDs that haven been skipped because they already exist on the server
	Skipped              []string `protobuf:"bytes,2,rep,name=skipped,proto3" json:"skipped,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GnuPGPublicKeyCreateResponse) Reset()         { *m = GnuPGPublicKeyCreateResponse{} }
func (m *GnuPGPublicKeyCreateResponse) String() string { return proto.CompactTextString(m) }
func (*GnuPGPublicKeyCreateResponse) ProtoMessage()    {}
func (*GnuPGPublicKeyCreateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8ba55a5eb76dc6fd, []int{2}
}
func (m *GnuPGPublicKeyCreateResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GnuPGPublicKeyCreateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GnuPGPublicKeyCreateResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GnuPGPublicKeyCreateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GnuPGPublicKeyCreateResponse.Merge(m, src)
}
func (m *GnuPGPublicKeyCreateResponse) XXX_Size() int {
	return m.Size()
}
func (m *GnuPGPublicKeyCreateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GnuPGPublicKeyCreateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GnuPGPublicKeyCreateResponse proto.InternalMessageInfo

func (m *GnuPGPublicKeyCreateResponse) GetCreated() *v1alpha1.GnuPGPublicKeyList {
	if m != nil {
		return m.Created
	}
	return nil
}

func (m *GnuPGPublicKeyCreateResponse) GetSkipped() []string {
	if m != nil {
		return m.Skipped
	}
	return nil
}

// Generic (empty) response for GPG public key CRUD requests
type GnuPGPublicKeyResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GnuPGPublicKeyResponse) Reset()         { *m = GnuPGPublicKeyResponse{} }
func (m *GnuPGPublicKeyResponse) String() string { return proto.CompactTextString(m) }
func (*GnuPGPublicKeyResponse) ProtoMessage()    {}
func (*GnuPGPublicKeyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8ba55a5eb76dc6fd, []int{3}
}
func (m *GnuPGPublicKeyResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GnuPGPublicKeyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GnuPGPublicKeyResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GnuPGPublicKeyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GnuPGPublicKeyResponse.Merge(m, src)
}
func (m *GnuPGPublicKeyResponse) XXX_Size() int {
	return m.Size()
}
func (m *GnuPGPublicKeyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GnuPGPublicKeyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GnuPGPublicKeyResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*GnuPGPublicKeyQuery)(nil), "gpgkey.GnuPGPublicKeyQuery")
	proto.RegisterType((*GnuPGPublicKeyCreateRequest)(nil), "gpgkey.GnuPGPublicKeyCreateRequest")
	proto.RegisterType((*GnuPGPublicKeyCreateResponse)(nil), "gpgkey.GnuPGPublicKeyCreateResponse")
	proto.RegisterType((*GnuPGPublicKeyResponse)(nil), "gpgkey.GnuPGPublicKeyResponse")
}

func init() { proto.RegisterFile("server/gpgkey/gpgkey.proto", fileDescriptor_8ba55a5eb76dc6fd) }

var fileDescriptor_8ba55a5eb76dc6fd = []byte{
	// 487 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x93, 0x41, 0x8b, 0xd3, 0x40,
	0x14, 0xc7, 0x99, 0xee, 0xda, 0xb5, 0x23, 0x22, 0x8e, 0xb2, 0x1b, 0xb3, 0xa5, 0x96, 0xe8, 0xa1,
	0x28, 0xce, 0xd0, 0xed, 0xcd, 0x9b, 0xba, 0x10, 0x64, 0x7b, 0xa8, 0xf1, 0xe6, 0x41, 0x49, 0x93,
	0x47, 0x76, 0x9a, 0x98, 0x19, 0x67, 0x26, 0x91, 0x20, 0x5e, 0xbc, 0x2b, 0x88, 0x9f, 0x40, 0xf0,
	0xc3, 0x78, 0x14, 0xfc, 0x02, 0x52, 0xfc, 0x20, 0xd2, 0x49, 0xea, 0x6e, 0x4b, 0x59, 0x3d, 0xf4,
	0x94, 0x79, 0x99, 0x79, 0xef, 0xff, 0x9b, 0xf7, 0xfe, 0x83, 0x5d, 0x0d, 0xaa, 0x04, 0xc5, 0x12,
	0x99, 0xa4, 0x50, 0x35, 0x1f, 0x2a, 0x95, 0x30, 0x82, 0xb4, 0xeb, 0xc8, 0xed, 0x26, 0x42, 0x24,
	0x19, 0xb0, 0x50, 0x72, 0x16, 0xe6, 0xb9, 0x30, 0xa1, 0xe1, 0x22, 0xd7, 0xf5, 0x29, 0x77, 0x9c,
	0x70, 0x73, 0x5a, 0x4c, 0x69, 0x24, 0x5e, 0xb3, 0x50, 0x25, 0x42, 0x2a, 0x31, 0xb3, 0x8b, 0x07,
	0x51, 0xcc, 0xca, 0x11, 0x93, 0x69, 0xb2, 0xc8, 0xd4, 0x2c, 0x94, 0x32, 0xe3, 0x91, 0xcd, 0x65,
	0xe5, 0x30, 0xcc, 0xe4, 0x69, 0x38, 0x64, 0x09, 0xe4, 0xa0, 0x42, 0x03, 0x71, 0x5d, 0xcd, 0xbb,
	0x8f, 0x6f, 0xf8, 0x79, 0x31, 0xf1, 0x27, 0xc5, 0x34, 0xe3, 0xd1, 0x09, 0x54, 0xcf, 0x0a, 0x50,
	0x15, 0xb9, 0x89, 0x2f, 0xa5, 0x50, 0x3d, 0x3d, 0x76, 0x50, 0x1f, 0x0d, 0x3a, 0x41, 0x1d, 0x78,
	0x5f, 0x11, 0x3e, 0x5c, 0x3d, 0xfd, 0x44, 0x41, 0x68, 0x20, 0x80, 0x37, 0x05, 0x68, 0x43, 0x66,
	0xb8, 0x23, 0xed, 0x4e, 0x0a, 0x95, 0xcd, 0xbc, 0x72, 0x34, 0xa6, 0x67, 0xb8, 0x74, 0x89, 0x6b,
	0x17, 0xaf, 0xa2, 0x98, 0x96, 0x23, 0x2a, 0xd3, 0x84, 0x2e, 0x70, 0xe9, 0x39, 0x5c, 0xba, 0xc4,
	0xa5, 0xab, 0x6a, 0xc1, 0x59, 0x79, 0xb2, 0x8f, 0xdb, 0x85, 0xd4, 0xa0, 0x8c, 0xd3, 0xea, 0xa3,
	0xc1, 0xe5, 0xa0, 0x89, 0xbc, 0x6f, 0x08, 0x77, 0x37, 0x33, 0x6a, 0x29, 0x72, 0x0d, 0x64, 0x86,
	0xf7, 0x22, 0xfb, 0x27, 0x6e, 0x10, 0x27, 0xdb, 0x44, 0x1c, 0x73, 0x6d, 0x82, 0xa5, 0x00, 0x71,
	0xf0, 0x9e, 0x4e, 0xb9, 0x94, 0x10, 0x3b, 0xad, 0xfe, 0xce, 0xa0, 0x13, 0x2c, 0x43, 0xcf, 0xc1,
	0xfb, 0x6b, 0x77, 0x6b, 0xf8, 0x8e, 0x3e, 0xee, 0xe2, 0xab, 0xfe, 0xc4, 0x3f, 0x81, 0xea, 0x39,
	0xa8, 0x92, 0x47, 0x40, 0x3e, 0x21, 0xbc, 0xbb, 0xa8, 0x4b, 0x0e, 0x69, 0xe3, 0x97, 0x0d, 0x23,
	0x73, 0xb7, 0x7e, 0x0d, 0xef, 0xe0, 0xc3, 0xcf, 0xdf, 0x5f, 0x5a, 0xd7, 0xc9, 0x35, 0xeb, 0xc4,
	0x72, 0xd8, 0xb8, 0x55, 0x93, 0xcf, 0x08, 0xef, 0xf8, 0xf0, 0x0f, 0x9e, 0xad, 0x4e, 0xde, 0xbb,
	0x6d, 0x59, 0x6e, 0x91, 0x83, 0x35, 0x16, 0xf6, 0xce, 0x5a, 0xf3, 0x3d, 0x79, 0x8b, 0xdb, 0xf5,
	0xa0, 0xc9, 0x9d, 0xcd, 0x54, 0x2b, 0x56, 0x75, 0xef, 0x5e, 0x7c, 0xa8, 0x9e, 0x85, 0xe7, 0x59,
	0xd5, 0xae, 0xb7, 0xde, 0x81, 0x87, 0xe7, 0x8c, 0xf8, 0x12, 0xb7, 0x8f, 0x21, 0x03, 0x03, 0x17,
	0xb7, 0xa3, 0xb7, 0x79, 0xf3, 0xaf, 0x54, 0xd3, 0xec, 0x7b, 0xeb, 0x52, 0x8f, 0x1f, 0x7d, 0x9f,
	0xf7, 0xd0, 0x8f, 0x79, 0x0f, 0xfd, 0x9a, 0xf7, 0xd0, 0x8b, 0xd1, 0xff, 0xbd, 0xfe, 0x28, 0xe3,
	0x90, 0x9b, 0xa6, 0xc6, 0xb4, 0x6d, 0xdf, 0xfa, 0xe8, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x44,
	0x30, 0xc0, 0xc7, 0x7d, 0x04, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// GPGKeyServiceClient is the client API for GPGKeyService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GPGKeyServiceClient interface {
	// List all available repository certificates
	List(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*v1alpha1.GnuPGPublicKeyList, error)
	// Get information about specified GPG public key from the server
	Get(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*v1alpha1.GnuPGPublicKey, error)
	// Create one or more GPG public keys in the server's configuration
	Create(ctx context.Context, in *GnuPGPublicKeyCreateRequest, opts ...grpc.CallOption) (*GnuPGPublicKeyCreateResponse, error)
	// Delete specified GPG public key from the server's configuration
	Delete(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*GnuPGPublicKeyResponse, error)
}

type gPGKeyServiceClient struct {
	cc *grpc.ClientConn
}

func NewGPGKeyServiceClient(cc *grpc.ClientConn) GPGKeyServiceClient {
	return &gPGKeyServiceClient{cc}
}

func (c *gPGKeyServiceClient) List(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*v1alpha1.GnuPGPublicKeyList, error) {
	out := new(v1alpha1.GnuPGPublicKeyList)
	err := c.cc.Invoke(ctx, "/gpgkey.GPGKeyService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gPGKeyServiceClient) Get(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*v1alpha1.GnuPGPublicKey, error) {
	out := new(v1alpha1.GnuPGPublicKey)
	err := c.cc.Invoke(ctx, "/gpgkey.GPGKeyService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gPGKeyServiceClient) Create(ctx context.Context, in *GnuPGPublicKeyCreateRequest, opts ...grpc.CallOption) (*GnuPGPublicKeyCreateResponse, error) {
	out := new(GnuPGPublicKeyCreateResponse)
	err := c.cc.Invoke(ctx, "/gpgkey.GPGKeyService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gPGKeyServiceClient) Delete(ctx context.Context, in *GnuPGPublicKeyQuery, opts ...grpc.CallOption) (*GnuPGPublicKeyResponse, error) {
	out := new(GnuPGPublicKeyResponse)
	err := c.cc.Invoke(ctx, "/gpgkey.GPGKeyService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GPGKeyServiceServer is the server API for GPGKeyService service.
type GPGKeyServiceServer interface {
	// List all available repository certificates
	List(context.Context, *GnuPGPublicKeyQuery) (*v1alpha1.GnuPGPublicKeyList, error)
	// Get information about specified GPG public key from the server
	Get(context.Context, *GnuPGPublicKeyQuery) (*v1alpha1.GnuPGPublicKey, error)
	// Create one or more GPG public keys in the server's configuration
	Create(context.Context, *GnuPGPublicKeyCreateRequest) (*GnuPGPublicKeyCreateResponse, error)
	// Delete specified GPG public key from the server's configuration
	Delete(context.Context, *GnuPGPublicKeyQuery) (*GnuPGPublicKeyResponse, error)
}

// UnimplementedGPGKeyServiceServer can be embedded to have forward compatible implementations.
type UnimplementedGPGKeyServiceServer struct {
}

func (*UnimplementedGPGKeyServiceServer) List(ctx context.Context, req *GnuPGPublicKeyQuery) (*v1alpha1.GnuPGPublicKeyList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (*UnimplementedGPGKeyServiceServer) Get(ctx context.Context, req *GnuPGPublicKeyQuery) (*v1alpha1.GnuPGPublicKey, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (*UnimplementedGPGKeyServiceServer) Create(ctx context.Context, req *GnuPGPublicKeyCreateRequest) (*GnuPGPublicKeyCreateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (*UnimplementedGPGKeyServiceServer) Delete(ctx context.Context, req *GnuPGPublicKeyQuery) (*GnuPGPublicKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}

func RegisterGPGKeyServiceServer(s *grpc.Server, srv GPGKeyServiceServer) {
	s.RegisterService(&_GPGKeyService_serviceDesc, srv)
}

func _GPGKeyService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GnuPGPublicKeyQuery)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GPGKeyServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gpgkey.GPGKeyService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GPGKeyServiceServer).List(ctx, req.(*GnuPGPublicKeyQuery))
	}
	return interceptor(ctx, in, info, handler)
}

func _GPGKeyService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GnuPGPublicKeyQuery)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GPGKeyServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gpgkey.GPGKeyService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GPGKeyServiceServer).Get(ctx, req.(*GnuPGPublicKeyQuery))
	}
	return interceptor(ctx, in, info, handler)
}

func _GPGKeyService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GnuPGPublicKeyCreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GPGKeyServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gpgkey.GPGKeyService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GPGKeyServiceServer).Create(ctx, req.(*GnuPGPublicKeyCreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GPGKeyService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GnuPGPublicKeyQuery)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GPGKeyServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gpgkey.GPGKeyService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GPGKeyServiceServer).Delete(ctx, req.(*GnuPGPublicKeyQuery))
	}
	return interceptor(ctx, in, info, handler)
}

var _GPGKeyService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "gpgkey.GPGKeyService",
	HandlerType: (*GPGKeyServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "List",
			Handler:    _GPGKeyService_List_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _GPGKeyService_Get_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _GPGKeyService_Create_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _GPGKeyService_Delete_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "server/gpgkey/gpgkey.proto",
}

func (m *GnuPGPublicKeyQuery) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GnuPGPublicKeyQuery) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GnuPGPublicKeyQuery) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.KeyID) > 0 {
		i -= len(m.KeyID)
		copy(dAtA[i:], m.KeyID)
		i = encodeVarintGpgkey(dAtA, i, uint64(len(m.KeyID)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *GnuPGPublicKeyCreateRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GnuPGPublicKeyCreateRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GnuPGPublicKeyCreateRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Upsert {
		i--
		if m.Upsert {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x10
	}
	if m.Publickey != nil {
		{
			size, err := m.Publickey.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGpgkey(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *GnuPGPublicKeyCreateResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GnuPGPublicKeyCreateResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GnuPGPublicKeyCreateResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Skipped) > 0 {
		for iNdEx := len(m.Skipped) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Skipped[iNdEx])
			copy(dAtA[i:], m.Skipped[iNdEx])
			i = encodeVarintGpgkey(dAtA, i, uint64(len(m.Skipped[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if m.Created != nil {
		{
			size, err := m.Created.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGpgkey(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *GnuPGPublicKeyResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GnuPGPublicKeyResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GnuPGPublicKeyResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func encodeVarintGpgkey(dAtA []byte, offset int, v uint64) int {
	offset -= sovGpgkey(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *GnuPGPublicKeyQuery) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.KeyID)
	if l > 0 {
		n += 1 + l + sovGpgkey(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *GnuPGPublicKeyCreateRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Publickey != nil {
		l = m.Publickey.Size()
		n += 1 + l + sovGpgkey(uint64(l))
	}
	if m.Upsert {
		n += 2
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *GnuPGPublicKeyCreateResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Created != nil {
		l = m.Created.Size()
		n += 1 + l + sovGpgkey(uint64(l))
	}
	if len(m.Skipped) > 0 {
		for _, s := range m.Skipped {
			l = len(s)
			n += 1 + l + sovGpgkey(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *GnuPGPublicKeyResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovGpgkey(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozGpgkey(x uint64) (n int) {
	return sovGpgkey(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *GnuPGPublicKeyQuery) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGpgkey
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GnuPGPublicKeyQuery: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GnuPGPublicKeyQuery: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field KeyID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGpgkey
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGpgkey
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.KeyID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGpgkey(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGpgkey
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *GnuPGPublicKeyCreateRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGpgkey
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GnuPGPublicKeyCreateRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GnuPGPublicKeyCreateRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Publickey", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGpgkey
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGpgkey
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Publickey == nil {
				m.Publickey = &v1alpha1.GnuPGPublicKey{}
			}
			if err := m.Publickey.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Upsert", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Upsert = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipGpgkey(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGpgkey
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *GnuPGPublicKeyCreateResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGpgkey
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GnuPGPublicKeyCreateResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GnuPGPublicKeyCreateResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Created", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGpgkey
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGpgkey
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Created == nil {
				m.Created = &v1alpha1.GnuPGPublicKeyList{}
			}
			if err := m.Created.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Skipped", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGpgkey
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGpgkey
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Skipped = append(m.Skipped, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGpgkey(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGpgkey
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *GnuPGPublicKeyResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGpgkey
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GnuPGPublicKeyResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GnuPGPublicKeyResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipGpgkey(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGpgkey
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipGpgkey(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowGpgkey
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGpgkey
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthGpgkey
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupGpgkey
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthGpgkey
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthGpgkey        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowGpgkey          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupGpgkey = fmt.Errorf("proto: unexpected end of group")
)

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: auth/v1/rpc_revoke_refresh_tokens.proto

package pb

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RevokeRefreshTokensRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RevokeRefreshTokensRequest) Reset() {
	*x = RevokeRefreshTokensRequest{}
	mi := &file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RevokeRefreshTokensRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeRefreshTokensRequest) ProtoMessage() {}

func (x *RevokeRefreshTokensRequest) ProtoReflect() protoreflect.Message {
	mi := &file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeRefreshTokensRequest.ProtoReflect.Descriptor instead.
func (*RevokeRefreshTokensRequest) Descriptor() ([]byte, []int) {
	return file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescGZIP(), []int{0}
}

func (x *RevokeRefreshTokensRequest) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

type RevokeRefreshTokensResponse struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	NumSessionsRevoked int64                  `protobuf:"varint,1,opt,name=num_sessions_revoked,json=numSessionsRevoked,proto3" json:"num_sessions_revoked,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *RevokeRefreshTokensResponse) Reset() {
	*x = RevokeRefreshTokensResponse{}
	mi := &file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RevokeRefreshTokensResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeRefreshTokensResponse) ProtoMessage() {}

func (x *RevokeRefreshTokensResponse) ProtoReflect() protoreflect.Message {
	mi := &file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeRefreshTokensResponse.ProtoReflect.Descriptor instead.
func (*RevokeRefreshTokensResponse) Descriptor() ([]byte, []int) {
	return file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescGZIP(), []int{1}
}

func (x *RevokeRefreshTokensResponse) GetNumSessionsRevoked() int64 {
	if x != nil {
		return x.NumSessionsRevoked
	}
	return 0
}

var File_auth_v1_rpc_revoke_refresh_tokens_proto protoreflect.FileDescriptor

const file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDesc = "" +
	"\n" +
	"'auth/v1/rpc_revoke_refresh_tokens.proto\x12\aauth.v1\x1a\x1bbuf/validate/validate.proto\"B\n" +
	"\x1aRevokeRefreshTokensRequest\x12$\n" +
	"\auser_id\x18\x01 \x01(\tB\v\xbaH\b\xc8\x01\x01r\x03\xb0\x01\x01R\x06userId\"O\n" +
	"\x1bRevokeRefreshTokensResponse\x120\n" +
	"\x14num_sessions_revoked\x18\x01 \x01(\x03R\x12numSessionsRevokedB\x9b\x01\n" +
	"\vcom.auth.v1B\x1bRpcRevokeRefreshTokensProtoP\x01Z2github.com/spazzle-io/spazzle-api/services/auth/pb\xa2\x02\x03AXX\xaa\x02\aAuth.V1\xca\x02\aAuth\\V1\xe2\x02\x13Auth\\V1\\GPBMetadata\xea\x02\bAuth::V1b\x06proto3"

var (
	file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescOnce sync.Once
	file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescData []byte
)

func file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescGZIP() []byte {
	file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescOnce.Do(func() {
		file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDesc), len(file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDesc)))
	})
	return file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDescData
}

var file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_auth_v1_rpc_revoke_refresh_tokens_proto_goTypes = []any{
	(*RevokeRefreshTokensRequest)(nil),  // 0: auth.v1.RevokeRefreshTokensRequest
	(*RevokeRefreshTokensResponse)(nil), // 1: auth.v1.RevokeRefreshTokensResponse
}
var file_auth_v1_rpc_revoke_refresh_tokens_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_auth_v1_rpc_revoke_refresh_tokens_proto_init() }
func file_auth_v1_rpc_revoke_refresh_tokens_proto_init() {
	if File_auth_v1_rpc_revoke_refresh_tokens_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDesc), len(file_auth_v1_rpc_revoke_refresh_tokens_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_auth_v1_rpc_revoke_refresh_tokens_proto_goTypes,
		DependencyIndexes: file_auth_v1_rpc_revoke_refresh_tokens_proto_depIdxs,
		MessageInfos:      file_auth_v1_rpc_revoke_refresh_tokens_proto_msgTypes,
	}.Build()
	File_auth_v1_rpc_revoke_refresh_tokens_proto = out.File
	file_auth_v1_rpc_revoke_refresh_tokens_proto_goTypes = nil
	file_auth_v1_rpc_revoke_refresh_tokens_proto_depIdxs = nil
}

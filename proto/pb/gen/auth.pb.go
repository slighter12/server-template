// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.2
// 	protoc        v5.29.2
// source: auth.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/gofeaturespb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RegisterRequest struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Email       *string                `protobuf:"bytes,1,opt,name=email" json:"email,omitempty"`
	xxx_hidden_Password    *string                `protobuf:"bytes,2,opt,name=password" json:"password,omitempty"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *RegisterRequest) Reset() {
	*x = RegisterRequest{}
	mi := &file_auth_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegisterRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterRequest) ProtoMessage() {}

func (x *RegisterRequest) ProtoReflect() protoreflect.Message {
	mi := &file_auth_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *RegisterRequest) GetEmail() string {
	if x != nil {
		if x.xxx_hidden_Email != nil {
			return *x.xxx_hidden_Email
		}
		return ""
	}
	return ""
}

func (x *RegisterRequest) GetPassword() string {
	if x != nil {
		if x.xxx_hidden_Password != nil {
			return *x.xxx_hidden_Password
		}
		return ""
	}
	return ""
}

func (x *RegisterRequest) SetEmail(v string) {
	x.xxx_hidden_Email = &v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 2)
}

func (x *RegisterRequest) SetPassword(v string) {
	x.xxx_hidden_Password = &v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 1, 2)
}

func (x *RegisterRequest) HasEmail() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *RegisterRequest) HasPassword() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 1)
}

func (x *RegisterRequest) ClearEmail() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Email = nil
}

func (x *RegisterRequest) ClearPassword() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 1)
	x.xxx_hidden_Password = nil
}

type RegisterRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Email    *string
	Password *string
}

func (b0 RegisterRequest_builder) Build() *RegisterRequest {
	m0 := &RegisterRequest{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Email != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 2)
		x.xxx_hidden_Email = b.Email
	}
	if b.Password != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 1, 2)
		x.xxx_hidden_Password = b.Password
	}
	return m0
}

type RegisterResponse struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Status *Status                `protobuf:"bytes,1,opt,name=status" json:"status,omitempty"`
	xxx_hidden_User   *User                  `protobuf:"bytes,2,opt,name=user" json:"user,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *RegisterResponse) Reset() {
	*x = RegisterResponse{}
	mi := &file_auth_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegisterResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterResponse) ProtoMessage() {}

func (x *RegisterResponse) ProtoReflect() protoreflect.Message {
	mi := &file_auth_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *RegisterResponse) GetStatus() *Status {
	if x != nil {
		return x.xxx_hidden_Status
	}
	return nil
}

func (x *RegisterResponse) GetUser() *User {
	if x != nil {
		return x.xxx_hidden_User
	}
	return nil
}

func (x *RegisterResponse) SetStatus(v *Status) {
	x.xxx_hidden_Status = v
}

func (x *RegisterResponse) SetUser(v *User) {
	x.xxx_hidden_User = v
}

func (x *RegisterResponse) HasStatus() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Status != nil
}

func (x *RegisterResponse) HasUser() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_User != nil
}

func (x *RegisterResponse) ClearStatus() {
	x.xxx_hidden_Status = nil
}

func (x *RegisterResponse) ClearUser() {
	x.xxx_hidden_User = nil
}

type RegisterResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Status *Status
	User   *User
}

func (b0 RegisterResponse_builder) Build() *RegisterResponse {
	m0 := &RegisterResponse{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Status = b.Status
	x.xxx_hidden_User = b.User
	return m0
}

type User struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Id          *string                `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	xxx_hidden_Email       *string                `protobuf:"bytes,2,opt,name=email" json:"email,omitempty"`
	xxx_hidden_CreatedAt   *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=created_at,json=createdAt" json:"created_at,omitempty"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *User) Reset() {
	*x = User{}
	mi := &file_auth_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *User) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*User) ProtoMessage() {}

func (x *User) ProtoReflect() protoreflect.Message {
	mi := &file_auth_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *User) GetId() string {
	if x != nil {
		if x.xxx_hidden_Id != nil {
			return *x.xxx_hidden_Id
		}
		return ""
	}
	return ""
}

func (x *User) GetEmail() string {
	if x != nil {
		if x.xxx_hidden_Email != nil {
			return *x.xxx_hidden_Email
		}
		return ""
	}
	return ""
}

func (x *User) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.xxx_hidden_CreatedAt
	}
	return nil
}

func (x *User) SetId(v string) {
	x.xxx_hidden_Id = &v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 3)
}

func (x *User) SetEmail(v string) {
	x.xxx_hidden_Email = &v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 1, 3)
}

func (x *User) SetCreatedAt(v *timestamppb.Timestamp) {
	x.xxx_hidden_CreatedAt = v
}

func (x *User) HasId() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *User) HasEmail() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 1)
}

func (x *User) HasCreatedAt() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_CreatedAt != nil
}

func (x *User) ClearId() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Id = nil
}

func (x *User) ClearEmail() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 1)
	x.xxx_hidden_Email = nil
}

func (x *User) ClearCreatedAt() {
	x.xxx_hidden_CreatedAt = nil
}

type User_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Id        *string
	Email     *string
	CreatedAt *timestamppb.Timestamp
}

func (b0 User_builder) Build() *User {
	m0 := &User{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Id != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 3)
		x.xxx_hidden_Id = b.Id
	}
	if b.Email != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 1, 3)
		x.xxx_hidden_Email = b.Email
	}
	x.xxx_hidden_CreatedAt = b.CreatedAt
	return m0
}

type Status struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Code        int32                  `protobuf:"varint,1,opt,name=code" json:"code,omitempty"`
	xxx_hidden_Message     *string                `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *Status) Reset() {
	*x = Status{}
	mi := &file_auth_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_auth_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *Status) GetCode() int32 {
	if x != nil {
		return x.xxx_hidden_Code
	}
	return 0
}

func (x *Status) GetMessage() string {
	if x != nil {
		if x.xxx_hidden_Message != nil {
			return *x.xxx_hidden_Message
		}
		return ""
	}
	return ""
}

func (x *Status) SetCode(v int32) {
	x.xxx_hidden_Code = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 2)
}

func (x *Status) SetMessage(v string) {
	x.xxx_hidden_Message = &v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 1, 2)
}

func (x *Status) HasCode() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *Status) HasMessage() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 1)
}

func (x *Status) ClearCode() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Code = 0
}

func (x *Status) ClearMessage() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 1)
	x.xxx_hidden_Message = nil
}

type Status_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Code    *int32
	Message *string
}

func (b0 Status_builder) Build() *Status {
	m0 := &Status{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Code != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 2)
		x.xxx_hidden_Code = *b.Code
	}
	if b.Message != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 1, 2)
		x.xxx_hidden_Message = b.Message
	}
	return m0
}

var File_auth_proto protoreflect.FileDescriptor

var file_auth_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x61, 0x75, 0x74, 0x68, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x61, 0x75,
	0x74, 0x68, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x67, 0x6f, 0x5f, 0x66, 0x65, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x43, 0x0a, 0x0f, 0x52, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05,
	0x65, 0x6d, 0x61, 0x69, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x6d, 0x61,
	0x69, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x22, 0x5e,
	0x0a, 0x10, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x27, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x21, 0x0a, 0x04, 0x75,
	0x73, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x61, 0x75, 0x74, 0x68,
	0x2e, 0x76, 0x31, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x04, 0x75, 0x73, 0x65, 0x72, 0x22, 0x67,
	0x0a, 0x04, 0x55, 0x73, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x12, 0x39, 0x0a, 0x0a,
	0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x63, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x22, 0x36, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x32,
	0x47, 0x0a, 0x04, 0x41, 0x75, 0x74, 0x68, 0x12, 0x3f, 0x0a, 0x08, 0x52, 0x65, 0x67, 0x69, 0x73,
	0x74, 0x65, 0x72, 0x12, 0x18, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65,
	0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e,
	0x61, 0x75, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x22, 0x5a, 0x18, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x2d, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x70, 0x62, 0x92, 0x03, 0x05, 0xd2, 0x3e, 0x02, 0x10, 0x03, 0x62, 0x08, 0x65, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x70, 0xe8, 0x07,
}

var file_auth_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_auth_proto_goTypes = []any{
	(*RegisterRequest)(nil),       // 0: auth.v1.RegisterRequest
	(*RegisterResponse)(nil),      // 1: auth.v1.RegisterResponse
	(*User)(nil),                  // 2: auth.v1.User
	(*Status)(nil),                // 3: auth.v1.Status
	(*timestamppb.Timestamp)(nil), // 4: google.protobuf.Timestamp
}
var file_auth_proto_depIdxs = []int32{
	3, // 0: auth.v1.RegisterResponse.status:type_name -> auth.v1.Status
	2, // 1: auth.v1.RegisterResponse.user:type_name -> auth.v1.User
	4, // 2: auth.v1.User.created_at:type_name -> google.protobuf.Timestamp
	0, // 3: auth.v1.Auth.Register:input_type -> auth.v1.RegisterRequest
	1, // 4: auth.v1.Auth.Register:output_type -> auth.v1.RegisterResponse
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_auth_proto_init() }
func file_auth_proto_init() {
	if File_auth_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_auth_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_auth_proto_goTypes,
		DependencyIndexes: file_auth_proto_depIdxs,
		MessageInfos:      file_auth_proto_msgTypes,
	}.Build()
	File_auth_proto = out.File
	file_auth_proto_rawDesc = nil
	file_auth_proto_goTypes = nil
	file_auth_proto_depIdxs = nil
}

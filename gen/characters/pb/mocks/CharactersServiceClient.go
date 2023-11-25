// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	pb "github.com/walkline/ToCloud9/gen/characters/pb"
)

// CharactersServiceClient is an autogenerated mock type for the CharactersServiceClient type
type CharactersServiceClient struct {
	mock.Mock
}

// AccountDataForAccount provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) AccountDataForAccount(ctx context.Context, in *pb.AccountDataForAccountRequest, opts ...grpc.CallOption) (*pb.AccountDataForAccountResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.AccountDataForAccountResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.AccountDataForAccountRequest, ...grpc.CallOption) (*pb.AccountDataForAccountResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.AccountDataForAccountRequest, ...grpc.CallOption) *pb.AccountDataForAccountResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.AccountDataForAccountResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.AccountDataForAccountRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CharacterByName provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) CharacterByName(ctx context.Context, in *pb.CharacterByNameRequest, opts ...grpc.CallOption) (*pb.CharacterByNameResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.CharacterByNameResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharacterByNameRequest, ...grpc.CallOption) (*pb.CharacterByNameResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharacterByNameRequest, ...grpc.CallOption) *pb.CharacterByNameResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.CharacterByNameResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.CharacterByNameRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CharacterOnlineByName provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) CharacterOnlineByName(ctx context.Context, in *pb.CharacterOnlineByNameRequest, opts ...grpc.CallOption) (*pb.CharacterOnlineByNameResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.CharacterOnlineByNameResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharacterOnlineByNameRequest, ...grpc.CallOption) (*pb.CharacterOnlineByNameResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharacterOnlineByNameRequest, ...grpc.CallOption) *pb.CharacterOnlineByNameResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.CharacterOnlineByNameResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.CharacterOnlineByNameRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CharactersToLoginByGUID provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) CharactersToLoginByGUID(ctx context.Context, in *pb.CharactersToLoginByGUIDRequest, opts ...grpc.CallOption) (*pb.CharactersToLoginByGUIDResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.CharactersToLoginByGUIDResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharactersToLoginByGUIDRequest, ...grpc.CallOption) (*pb.CharactersToLoginByGUIDResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharactersToLoginByGUIDRequest, ...grpc.CallOption) *pb.CharactersToLoginByGUIDResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.CharactersToLoginByGUIDResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.CharactersToLoginByGUIDRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CharactersToLoginForAccount provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) CharactersToLoginForAccount(ctx context.Context, in *pb.CharactersToLoginForAccountRequest, opts ...grpc.CallOption) (*pb.CharactersToLoginForAccountResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.CharactersToLoginForAccountResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharactersToLoginForAccountRequest, ...grpc.CallOption) (*pb.CharactersToLoginForAccountResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.CharactersToLoginForAccountRequest, ...grpc.CallOption) *pb.CharactersToLoginForAccountResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.CharactersToLoginForAccountResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.CharactersToLoginForAccountRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WhoQuery provides a mock function with given fields: ctx, in, opts
func (_m *CharactersServiceClient) WhoQuery(ctx context.Context, in *pb.WhoQueryRequest, opts ...grpc.CallOption) (*pb.WhoQueryResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.WhoQueryResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.WhoQueryRequest, ...grpc.CallOption) (*pb.WhoQueryResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.WhoQueryRequest, ...grpc.CallOption) *pb.WhoQueryResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.WhoQueryResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.WhoQueryRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewCharactersServiceClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewCharactersServiceClient creates a new instance of CharactersServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCharactersServiceClient(t mockConstructorTestingTNewCharactersServiceClient) *CharactersServiceClient {
	mock := &CharactersServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

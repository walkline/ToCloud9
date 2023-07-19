// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	grpc "google.golang.org/grpc"

	pb "github.com/walkline/ToCloud9/gen/mail/pb"
)

// MailServiceClient is an autogenerated mock type for the MailServiceClient type
type MailServiceClient struct {
	mock.Mock
}

// DeleteMail provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) DeleteMail(ctx context.Context, in *pb.DeleteMailRequest, opts ...grpc.CallOption) (*pb.DeleteMailResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.DeleteMailResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.DeleteMailRequest, ...grpc.CallOption) (*pb.DeleteMailResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.DeleteMailRequest, ...grpc.CallOption) *pb.DeleteMailResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.DeleteMailResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.DeleteMailRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MailByID provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) MailByID(ctx context.Context, in *pb.MailByIDRequest, opts ...grpc.CallOption) (*pb.MailByIDResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.MailByIDResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MailByIDRequest, ...grpc.CallOption) (*pb.MailByIDResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MailByIDRequest, ...grpc.CallOption) *pb.MailByIDResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.MailByIDResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.MailByIDRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MailsForPlayer provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) MailsForPlayer(ctx context.Context, in *pb.MailsForPlayerRequest, opts ...grpc.CallOption) (*pb.MailsForPlayerResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.MailsForPlayerResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MailsForPlayerRequest, ...grpc.CallOption) (*pb.MailsForPlayerResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MailsForPlayerRequest, ...grpc.CallOption) *pb.MailsForPlayerResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.MailsForPlayerResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.MailsForPlayerRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MarkAsReadForPlayer provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) MarkAsReadForPlayer(ctx context.Context, in *pb.MarkAsReadForPlayerRequest, opts ...grpc.CallOption) (*pb.MarkAsReadForPlayerResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.MarkAsReadForPlayerResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MarkAsReadForPlayerRequest, ...grpc.CallOption) (*pb.MarkAsReadForPlayerResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.MarkAsReadForPlayerRequest, ...grpc.CallOption) *pb.MarkAsReadForPlayerResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.MarkAsReadForPlayerResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.MarkAsReadForPlayerRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveMailItem provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) RemoveMailItem(ctx context.Context, in *pb.RemoveMailItemRequest, opts ...grpc.CallOption) (*pb.RemoveMailItemResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.RemoveMailItemResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.RemoveMailItemRequest, ...grpc.CallOption) (*pb.RemoveMailItemResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.RemoveMailItemRequest, ...grpc.CallOption) *pb.RemoveMailItemResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.RemoveMailItemResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.RemoveMailItemRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveMailMoney provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) RemoveMailMoney(ctx context.Context, in *pb.RemoveMailMoneyRequest, opts ...grpc.CallOption) (*pb.RemoveMailMoneyResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.RemoveMailMoneyResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.RemoveMailMoneyRequest, ...grpc.CallOption) (*pb.RemoveMailMoneyResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.RemoveMailMoneyRequest, ...grpc.CallOption) *pb.RemoveMailMoneyResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.RemoveMailMoneyResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.RemoveMailMoneyRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Send provides a mock function with given fields: ctx, in, opts
func (_m *MailServiceClient) Send(ctx context.Context, in *pb.SendRequest, opts ...grpc.CallOption) (*pb.SendResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pb.SendResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.SendRequest, ...grpc.CallOption) (*pb.SendResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.SendRequest, ...grpc.CallOption) *pb.SendResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.SendResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.SendRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMailServiceClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewMailServiceClient creates a new instance of MailServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMailServiceClient(t mockConstructorTestingTNewMailServiceClient) *MailServiceClient {
	mock := &MailServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

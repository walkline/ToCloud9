// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	events "github.com/walkline/ToCloud9/shared/events"
)

// ServerRegistryProducer is an autogenerated mock type for the ServerRegistryProducer type
type ServerRegistryProducer struct {
	mock.Mock
}

// GSAdded provides a mock function with given fields: payload
func (_m *ServerRegistryProducer) GSAdded(payload *events.ServerRegistryEventGSAddedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.ServerRegistryEventGSAddedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GSMapsReassigned provides a mock function with given fields: payload
func (_m *ServerRegistryProducer) GSMapsReassigned(payload *events.ServerRegistryEventGSMapsReassignedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.ServerRegistryEventGSMapsReassignedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GSRemoved provides a mock function with given fields: payload
func (_m *ServerRegistryProducer) GSRemoved(payload *events.ServerRegistryEventGSRemovedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.ServerRegistryEventGSRemovedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LBAdded provides a mock function with given fields: payload
func (_m *ServerRegistryProducer) LBAdded(payload *events.ServerRegistryEventLBAddedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.ServerRegistryEventLBAddedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LBRemovedUnhealthy provides a mock function with given fields: payload
func (_m *ServerRegistryProducer) LBRemovedUnhealthy(payload *events.ServerRegistryEventLBRemovedUnhealthyPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.ServerRegistryEventLBRemovedUnhealthyPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewServerRegistryProducer interface {
	mock.TestingT
	Cleanup(func())
}

// NewServerRegistryProducer creates a new instance of ServerRegistryProducer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewServerRegistryProducer(t mockConstructorTestingTNewServerRegistryProducer) *ServerRegistryProducer {
	mock := &ServerRegistryProducer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

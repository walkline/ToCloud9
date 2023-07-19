// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	events "github.com/walkline/ToCloud9/shared/events"
)

// MailServiceProducer is an autogenerated mock type for the MailServiceProducer type
type MailServiceProducer struct {
	mock.Mock
}

// IncomingMail provides a mock function with given fields: payload
func (_m *MailServiceProducer) IncomingMail(payload *events.MailEventIncomingMailPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.MailEventIncomingMailPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMailServiceProducer interface {
	mock.TestingT
	Cleanup(func())
}

// NewMailServiceProducer creates a new instance of MailServiceProducer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMailServiceProducer(t mockConstructorTestingTNewMailServiceProducer) *MailServiceProducer {
	mock := &MailServiceProducer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
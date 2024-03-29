// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	events "github.com/walkline/ToCloud9/shared/events"
)

// GuildServiceProducer is an autogenerated mock type for the GuildServiceProducer type
type GuildServiceProducer struct {
	mock.Mock
}

// GuildInfoUpdated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) GuildInfoUpdated(payload *events.GuildEventGuildInfoUpdatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventGuildInfoUpdatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InviteCreated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) InviteCreated(payload *events.GuildEventInviteCreatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventInviteCreatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MOTDUpdated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MOTDUpdated(payload *events.GuildEventMOTDUpdatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMOTDUpdatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberAdded provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberAdded(payload *events.GuildEventMemberAddedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMemberAddedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberDemote provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberDemote(payload *events.GuildEventMemberDemotePayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMemberDemotePayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberKicked provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberKicked(payload *events.GuildEventMemberKickedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMemberKickedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberLeft provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberLeft(payload *events.GuildEventMemberLeftPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMemberLeftPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberNoteUpdated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberNoteUpdated(payload *events.GuildEventMembersNoteUpdatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMembersNoteUpdatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberOfficerNoteUpdated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberOfficerNoteUpdated(payload *events.GuildEventMembersOfficerNoteUpdatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMembersOfficerNoteUpdatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberPromote provides a mock function with given fields: payload
func (_m *GuildServiceProducer) MemberPromote(payload *events.GuildEventMemberPromotePayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventMemberPromotePayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMessage provides a mock function with given fields: payload
func (_m *GuildServiceProducer) NewMessage(payload *events.GuildEventNewMessagePayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventNewMessagePayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RankCreated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) RankCreated(payload *events.GuildEventRankCreatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventRankCreatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RankDeleted provides a mock function with given fields: payload
func (_m *GuildServiceProducer) RankDeleted(payload *events.GuildEventRankDeletedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventRankDeletedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RankUpdated provides a mock function with given fields: payload
func (_m *GuildServiceProducer) RankUpdated(payload *events.GuildEventRankUpdatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GuildEventRankUpdatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewGuildServiceProducer interface {
	mock.TestingT
	Cleanup(func())
}

// NewGuildServiceProducer creates a new instance of GuildServiceProducer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewGuildServiceProducer(t mockConstructorTestingTNewGuildServiceProducer) *GuildServiceProducer {
	mock := &GuildServiceProducer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

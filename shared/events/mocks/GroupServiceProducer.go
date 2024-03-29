// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	events "github.com/walkline/ToCloud9/shared/events"
)

// GroupServiceProducer is an autogenerated mock type for the GroupServiceProducer type
type GroupServiceProducer struct {
	mock.Mock
}

// ConvertedToRaid provides a mock function with given fields: payload
func (_m *GroupServiceProducer) ConvertedToRaid(payload *events.GroupEventGroupConvertedToRaidPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupConvertedToRaidPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GroupCreated provides a mock function with given fields: payload
func (_m *GroupServiceProducer) GroupCreated(payload *events.GroupEventGroupCreatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupCreatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GroupDifficultyChanged provides a mock function with given fields: payload
func (_m *GroupServiceProducer) GroupDifficultyChanged(payload *events.GroupEventGroupDifficultyChangedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupDifficultyChangedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GroupDisband provides a mock function with given fields: payload
func (_m *GroupServiceProducer) GroupDisband(payload *events.GroupEventGroupDisbandPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupDisbandPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GroupMemberLeft provides a mock function with given fields: payload
func (_m *GroupServiceProducer) GroupMemberLeft(payload *events.GroupEventGroupMemberLeftPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupMemberLeftPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GroupMemberOnlineStatusChanged provides a mock function with given fields: payload
func (_m *GroupServiceProducer) GroupMemberOnlineStatusChanged(payload *events.GroupEventGroupMemberOnlineStatusChangedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupMemberOnlineStatusChangedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InviteCreated provides a mock function with given fields: payload
func (_m *GroupServiceProducer) InviteCreated(payload *events.GroupEventInviteCreatedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventInviteCreatedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LeaderChanged provides a mock function with given fields: payload
func (_m *GroupServiceProducer) LeaderChanged(payload *events.GroupEventGroupLeaderChangedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupLeaderChangedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LootTypeChanged provides a mock function with given fields: payload
func (_m *GroupServiceProducer) LootTypeChanged(payload *events.GroupEventGroupLootTypeChangedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupLootTypeChangedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MemberAdded provides a mock function with given fields: payload
func (_m *GroupServiceProducer) MemberAdded(payload *events.GroupEventGroupMemberAddedPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventGroupMemberAddedPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendChatMessage provides a mock function with given fields: payload
func (_m *GroupServiceProducer) SendChatMessage(payload *events.GroupEventNewMessagePayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventNewMessagePayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// TargetIconUpdated provides a mock function with given fields: payload
func (_m *GroupServiceProducer) TargetIconUpdated(payload *events.GroupEventNewTargetIconPayload) error {
	ret := _m.Called(payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(*events.GroupEventNewTargetIconPayload) error); ok {
		r0 = rf(payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewGroupServiceProducer interface {
	mock.TestingT
	Cleanup(func())
}

// NewGroupServiceProducer creates a new instance of GroupServiceProducer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewGroupServiceProducer(t mockConstructorTestingTNewGroupServiceProducer) *GroupServiceProducer {
	mock := &GroupServiceProducer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

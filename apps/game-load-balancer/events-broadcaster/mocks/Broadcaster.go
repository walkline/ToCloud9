// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	events_broadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	events "github.com/walkline/ToCloud9/shared/events"
)

// Broadcaster is an autogenerated mock type for the Broadcaster type
type Broadcaster struct {
	mock.Mock
}

// NewGuildInviteCreatedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildInviteCreatedEvent(payload *events_broadcaster.GuildInviteCreatedPayload) {
	_m.Called(payload)
}

// NewGuildMOTDUpdatedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMOTDUpdatedEvent(payload *events.GuildEventMOTDUpdatedPayload) {
	_m.Called(payload)
}

// NewGuildMemberAddedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMemberAddedEvent(payload *events.GuildEventMemberAddedPayload) {
	_m.Called(payload)
}

// NewGuildMemberDemoteEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMemberDemoteEvent(payload *events.GuildEventMemberDemotePayload) {
	_m.Called(payload)
}

// NewGuildMemberKickedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMemberKickedEvent(payload *events.GuildEventMemberKickedPayload) {
	_m.Called(payload)
}

// NewGuildMemberLeftEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMemberLeftEvent(payload *events.GuildEventMemberLeftPayload) {
	_m.Called(payload)
}

// NewGuildMemberPromoteEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMemberPromoteEvent(payload *events.GuildEventMemberPromotePayload) {
	_m.Called(payload)
}

// NewGuildMessageEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildMessageEvent(payload *events.GuildEventNewMessagePayload) {
	_m.Called(payload)
}

// NewGuildRankCreatedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildRankCreatedEvent(payload *events.GuildEventRankCreatedPayload) {
	_m.Called(payload)
}

// NewGuildRankDeletedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildRankDeletedEvent(payload *events.GuildEventRankDeletedPayload) {
	_m.Called(payload)
}

// NewGuildRankUpdatedEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewGuildRankUpdatedEvent(payload *events.GuildEventRankUpdatedPayload) {
	_m.Called(payload)
}

// NewIncomingMailEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewIncomingMailEvent(payload *events.MailEventIncomingMailPayload) {
	_m.Called(payload)
}

// NewIncomingWhisperEvent provides a mock function with given fields: payload
func (_m *Broadcaster) NewIncomingWhisperEvent(payload *events_broadcaster.IncomingWhisperPayload) {
	_m.Called(payload)
}

// RegisterCharacter provides a mock function with given fields: charGUID
func (_m *Broadcaster) RegisterCharacter(charGUID uint64) <-chan events_broadcaster.Event {
	ret := _m.Called(charGUID)

	var r0 <-chan events_broadcaster.Event
	if rf, ok := ret.Get(0).(func(uint64) <-chan events_broadcaster.Event); ok {
		r0 = rf(charGUID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan events_broadcaster.Event)
		}
	}

	return r0
}

// UnregisterCharacter provides a mock function with given fields: charGUID
func (_m *Broadcaster) UnregisterCharacter(charGUID uint64) {
	_m.Called(charGUID)
}

type mockConstructorTestingTNewBroadcaster interface {
	mock.TestingT
	Cleanup(func())
}

// NewBroadcaster creates a new instance of Broadcaster. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewBroadcaster(t mockConstructorTestingTNewBroadcaster) *Broadcaster {
	mock := &Broadcaster{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

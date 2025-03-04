// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/fkwhite/quic-go/logging (interfaces: Tracer)

// Package logging is a generated GoMock package.
package logging

import (
	context "context"
	net "net"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	protocol "github.com/fkwhite/quic-go/internal/protocol"
	wire "github.com/fkwhite/quic-go/internal/wire"
)

// MockTracer is a mock of Tracer interface.
type MockTracer struct {
	ctrl     *gomock.Controller
	recorder *MockTracerMockRecorder
}

// MockTracerMockRecorder is the mock recorder for MockTracer.
type MockTracerMockRecorder struct {
	mock *MockTracer
}

// NewMockTracer creates a new mock instance.
func NewMockTracer(ctrl *gomock.Controller) *MockTracer {
	mock := &MockTracer{ctrl: ctrl}
	mock.recorder = &MockTracerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTracer) EXPECT() *MockTracerMockRecorder {
	return m.recorder
}

// DroppedPacket mocks base method.
func (m *MockTracer) DroppedPacket(arg0 net.Addr, arg1 PacketType, arg2 protocol.ByteCount, arg3 PacketDropReason) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DroppedPacket", arg0, arg1, arg2, arg3)
}

// DroppedPacket indicates an expected call of DroppedPacket.
func (mr *MockTracerMockRecorder) DroppedPacket(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DroppedPacket", reflect.TypeOf((*MockTracer)(nil).DroppedPacket), arg0, arg1, arg2, arg3)
}

// SentPacket mocks base method.
func (m *MockTracer) SentPacket(arg0 net.Addr, arg1 *wire.Header, arg2 protocol.ByteCount, arg3 []Frame) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SentPacket", arg0, arg1, arg2, arg3)
}

// SentPacket indicates an expected call of SentPacket.
func (mr *MockTracerMockRecorder) SentPacket(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SentPacket", reflect.TypeOf((*MockTracer)(nil).SentPacket), arg0, arg1, arg2, arg3)
}

// SentVersionNegotiationPacket mocks base method.
func (m *MockTracer) SentVersionNegotiationPacket(arg0 net.Addr, arg1, arg2 protocol.ArbitraryLenConnectionID, arg3 []protocol.VersionNumber) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SentVersionNegotiationPacket", arg0, arg1, arg2, arg3)
}

// SentVersionNegotiationPacket indicates an expected call of SentVersionNegotiationPacket.
func (mr *MockTracerMockRecorder) SentVersionNegotiationPacket(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SentVersionNegotiationPacket", reflect.TypeOf((*MockTracer)(nil).SentVersionNegotiationPacket), arg0, arg1, arg2, arg3)
}

// TracerForConnection mocks base method.
func (m *MockTracer) TracerForConnection(arg0 context.Context, arg1 protocol.Perspective, arg2 protocol.ConnectionID) ConnectionTracer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TracerForConnection", arg0, arg1, arg2)
	ret0, _ := ret[0].(ConnectionTracer)
	return ret0
}

// TracerForConnection indicates an expected call of TracerForConnection.
func (mr *MockTracerMockRecorder) TracerForConnection(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TracerForConnection", reflect.TypeOf((*MockTracer)(nil).TracerForConnection), arg0, arg1, arg2)
}

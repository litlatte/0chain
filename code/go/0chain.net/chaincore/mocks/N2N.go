// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	datastore "0chain.net/core/datastore"
	mock "github.com/stretchr/testify/mock"

	node "0chain.net/chaincore/node"
)

// N2N is an autogenerated mock type for the N2N type
type N2N struct {
	mock.Mock
}

// RequestEntity provides a mock function with given fields: ctx, requestor, params, handler
func (_m *N2N) RequestEntity(ctx context.Context, requestor node.EntityRequestor, params map[string]string, handler datastore.JSONEntityReqResponderF) *node.Node {
	ret := _m.Called(ctx, requestor, params, handler)

	var r0 *node.Node
	if rf, ok := ret.Get(0).(func(context.Context, node.EntityRequestor, map[string]string, datastore.JSONEntityReqResponderF) *node.Node); ok {
		r0 = rf(ctx, requestor, params, handler)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*node.Node)
		}
	}

	return r0
}

// RequestEntityFromAll provides a mock function with given fields: ctx, requestor, params, handler
func (_m *N2N) RequestEntityFromAll(ctx context.Context, requestor node.EntityRequestor, params map[string]string, handler datastore.JSONEntityReqResponderF) {
	_m.Called(ctx, requestor, params, handler)
}

// RequestEntityFromNode provides a mock function with given fields: ctx, requestor, params, handler
func (_m *N2N) RequestEntityFromNode(ctx context.Context, requestor node.EntityRequestor, params map[string]string, handler datastore.JSONEntityReqResponderF) {
	_m.Called(ctx, requestor, params, handler)
}

// SendAll provides a mock function with given fields: handler
func (_m *N2N) SendAll(handler node.SendHandler) []*node.Node {
	ret := _m.Called(handler)

	var r0 []*node.Node
	if rf, ok := ret.Get(0).(func(node.SendHandler) []*node.Node); ok {
		r0 = rf(handler)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*node.Node)
		}
	}

	return r0
}

// SendTo provides a mock function with given fields: handler, to
func (_m *N2N) SendTo(handler node.SendHandler, to string) (bool, error) {
	ret := _m.Called(handler, to)

	var r0 bool
	if rf, ok := ret.Get(0).(func(node.SendHandler, string) bool); ok {
		r0 = rf(handler, to)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(node.SendHandler, string) error); ok {
		r1 = rf(handler, to)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

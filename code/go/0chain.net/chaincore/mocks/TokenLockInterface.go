// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// TokenLockInterface is an autogenerated mock type for the TokenLockInterface type
type TokenLockInterface struct {
	mock.Mock
}

// IsLocked provides a mock function with given fields: entity
func (_m *TokenLockInterface) IsLocked(entity interface{}) bool {
	ret := _m.Called(entity)

	var r0 bool
	if rf, ok := ret.Get(0).(func(interface{}) bool); ok {
		r0 = rf(entity)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// LockStats provides a mock function with given fields: entity
func (_m *TokenLockInterface) LockStats(entity interface{}) []byte {
	ret := _m.Called(entity)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(interface{}) []byte); ok {
		r0 = rf(entity)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

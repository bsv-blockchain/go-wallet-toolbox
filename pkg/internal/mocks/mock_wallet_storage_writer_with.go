package mocks

import (
	"testing"

	"go.uber.org/mock/gomock"
)

func (m *MockWalletStorageProvider) WithDefaultFindOrInsertUser(t testing.TB) *MockWalletStorageProvider {
	responses := DefaultResponses(t)
	responses.FindOrInsertUser.limitCallTimes(m.EXPECT().FindOrInsertUser(gomock.Any(), gomock.Any()).AnyTimes().Return(responses.FindOrInsertUser.result()))

	return m
}

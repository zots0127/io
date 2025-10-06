package mocks

import (
	"context"
	"io"
	
	"github.com/stretchr/testify/mock"
	"github.com/zots0127/io/internal/domain/entities"
)

// MockStorageRepository is a mock implementation of StorageRepository
type MockStorageRepository struct {
	mock.Mock
}

func (m *MockStorageRepository) Store(ctx context.Context, reader io.Reader) (string, error) {
	args := m.Called(ctx, reader)
	return args.String(0), args.Error(1)
}

func (m *MockStorageRepository) Retrieve(ctx context.Context, hash string) (io.ReadCloser, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorageRepository) Delete(ctx context.Context, hash string) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockStorageRepository) Exists(ctx context.Context, hash string) (bool, error) {
	args := m.Called(ctx, hash)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorageRepository) GetMetadata(ctx context.Context, hash string) (*entities.File, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.File), args.Error(1)
}

func (m *MockStorageRepository) ListFiles(ctx context.Context, limit, offset int) ([]*entities.File, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.File), args.Error(1)
}

func (m *MockStorageRepository) GetStorageStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}
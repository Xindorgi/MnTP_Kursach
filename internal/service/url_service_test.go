package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/v8950/url-shortener/internal/domain"
)

// MockURLRepository is a mock implementation of repository.URLRepository.
type MockURLRepository struct {
	mock.Mock
}

func (m *MockURLRepository) Insert(ctx context.Context, longURL string) (*domain.URL, error) {
	args := m.Called(ctx, longURL)
	return args.Get(0).(*domain.URL), args.Error(1)
}

func (m *MockURLRepository) FindByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	args := m.Called(ctx, shortCode)
	return args.Get(0).(*domain.URL), args.Error(1)
}

func (m *MockURLRepository) UpdateShortCode(ctx context.Context, id int64, shortCode string) error {
	args := m.Called(ctx, id, shortCode)
	return args.Error(0)
}

func (m *MockURLRepository) FindByManagementToken(ctx context.Context, token string) (*domain.URL, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*domain.URL), args.Error(1)
}

// MockClickRepository is a mock implementation of repository.ClickRepository.
type MockClickRepository struct {
	mock.Mock
}

func (m *MockClickRepository) BatchInsert(ctx context.Context, events []domain.ClickEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

func (m *MockClickRepository) GetStats(ctx context.Context, urlID int64) (*domain.ClickStats, error) {
	args := m.Called(ctx, urlID)
	return args.Get(0).(*domain.ClickStats), args.Error(1)
}

// MockCacheRepository is a mock implementation of repository.CacheRepository.
type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) Get(ctx context.Context, shortCode string) (string, error) {
	args := m.Called(ctx, shortCode)
	return args.String(0), args.Error(1)
}

func (m *MockCacheRepository) Set(ctx context.Context, shortCode, longURL string) error {
	args := m.Called(ctx, shortCode, longURL)
	return args.Error(0)
}

func (m *MockCacheRepository) Delete(ctx context.Context, shortCode string) error {
	args := m.Called(ctx, shortCode)
	return args.Error(0)
}

// newTestService creates a URLService with mocks for testing.
func newTestService(t *testing.T) (*URLService, *MockURLRepository, *MockClickRepository, *MockCacheRepository) {
	t.Helper()
	mockURLRepo := new(MockURLRepository)
	mockClickRepo := new(MockClickRepository)
	mockCacheRepo := new(MockCacheRepository)
	eventsChan := make(chan domain.ClickEvent, 100)

	svc, err := NewURLService(mockURLRepo, mockClickRepo, mockCacheRepo, eventsChan, "http://localhost:8080")
	assert.NoError(t, err)

	return svc, mockURLRepo, mockClickRepo, mockCacheRepo
}

func TestCreateShortURL_Success(t *testing.T) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(t)

	longURL := "https://example.com/very/long/url"
	expectedURL := &domain.URL{
		ID:              42,
		LongURL:         longURL,
		ManagementToken: "test-token-uuid",
	}

	mockURLRepo.On("Insert", mock.Anything, longURL).Return(expectedURL, nil)
	mockURLRepo.On("UpdateShortCode", mock.Anything, int64(42), mock.AnythingOfType("string")).Return(nil)
	mockCacheRepo.On("Set", mock.Anything, mock.AnythingOfType("string"), longURL).Return(nil)

	result, err := svc.CreateShortURL(context.Background(), longURL)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, longURL, result.LongURL)
	assert.NotEmpty(t, result.ShortCode)
	assert.Equal(t, "test-token-uuid", result.ManagementToken)

	mockURLRepo.AssertExpectations(t)
	mockCacheRepo.AssertExpectations(t)
}

func TestCreateShortURL_InsertError(t *testing.T) {
	svc, mockURLRepo, _, _ := newTestService(t)

	mockURLRepo.On("Insert", mock.Anything, mock.Anything).Return((*domain.URL)(nil), errors.New("db error"))

	result, err := svc.CreateShortURL(context.Background(), "https://example.com")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to insert URL")
}

func TestResolveURL_CacheHit(t *testing.T) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(t)

	shortCode := "abc123"
	longURL := "https://example.com"

	mockCacheRepo.On("Get", mock.Anything, shortCode).Return(longURL, nil)

	result, err := svc.ResolveURL(context.Background(), shortCode)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shortCode, result.ShortCode)
	assert.Equal(t, longURL, result.LongURL)

	// DB should NOT be called on cache hit
	mockURLRepo.AssertNotCalled(t, "FindByShortCode")
}

func TestResolveURL_CacheMiss(t *testing.T) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(t)

	shortCode := "abc123"
	longURL := "https://example.com"
	expectedURL := &domain.URL{
		ID:        42,
		LongURL:   longURL,
		ShortCode: shortCode,
	}

	mockCacheRepo.On("Get", mock.Anything, shortCode).Return("", errors.New("cache miss"))
	mockURLRepo.On("FindByShortCode", mock.Anything, shortCode).Return(expectedURL, nil)
	mockCacheRepo.On("Set", mock.Anything, shortCode, longURL).Return(nil)

	result, err := svc.ResolveURL(context.Background(), shortCode)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, longURL, result.LongURL)
	assert.Equal(t, int64(42), result.ID)
}

func TestResolveURL_NotFound(t *testing.T) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(t)

	mockCacheRepo.On("Get", mock.Anything, "nonexistent").Return("", errors.New("cache miss"))
	mockURLRepo.On("FindByShortCode", mock.Anything, "nonexistent").Return((*domain.URL)(nil), errors.New("not found"))

	result, err := svc.ResolveURL(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "URL not found")
}

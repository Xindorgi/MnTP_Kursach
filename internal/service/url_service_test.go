package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sqids/sqids-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Xindorgi/MnTP_Kursach/internal/domain"
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

// testHelper is an interface satisfied by both *testing.T and *testing.B.
type testHelper interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// newTestService creates a URLService with mocks for testing.
// Accepts both *testing.T and *testing.B via the testHelper interface.
func newTestService(t testHelper) (*URLService, *MockURLRepository, *MockClickRepository, *MockCacheRepository) {
	t.Helper()
	mockURLRepo := new(MockURLRepository)
	mockClickRepo := new(MockClickRepository)
	mockCacheRepo := new(MockCacheRepository)
	eventsChan := make(chan domain.ClickEvent, 100)

	svc, err := NewURLService(mockURLRepo, mockClickRepo, mockCacheRepo, eventsChan, "http://localhost:8080")
	if err != nil {
		t.Fatalf("failed to create URL service: %v", err)
	}

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
	assert.NotNil(t, result.ShortCode)
	assert.NotEmpty(t, *result.ShortCode)
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
	assert.NotNil(t, result.ShortCode)
	assert.Equal(t, shortCode, *result.ShortCode)
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
		ShortCode: &shortCode,
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

// Benchmarks

// BenchmarkCreateShortURL measures the performance of creating a short URL.
// This benchmarks the full flow: Insert → sqids.Encode → UpdateShortCode → Cache Set.
func BenchmarkCreateShortURL(b *testing.B) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(b)

	longURL := "https://example.com/very/long/url/for/benchmarking/" + time.Now().String()

	// Setup mocks to return quickly
	mockURLRepo.On("Insert", mock.Anything, mock.AnythingOfType("string")).Return(&domain.URL{
		ID:              42,
		LongURL:         longURL,
		ManagementToken: "benchmark-token",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil)
	mockURLRepo.On("UpdateShortCode", mock.Anything, int64(42), mock.AnythingOfType("string")).Return(nil)
	mockCacheRepo.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.CreateShortURL(context.Background(), longURL)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveURL_CacheHit measures redirect performance when URL is cached.
func BenchmarkResolveURL_CacheHit(b *testing.B) {
	svc, _, _, mockCacheRepo := newTestService(b)

	shortCode := "abc123"
	longURL := "https://example.com"

	mockCacheRepo.On("Get", mock.Anything, shortCode).Return(longURL, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.ResolveURL(context.Background(), shortCode)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveURL_CacheMiss measures redirect performance when cache is cold.
func BenchmarkResolveURL_CacheMiss(b *testing.B) {
	svc, mockURLRepo, _, mockCacheRepo := newTestService(b)

	shortCode := "abc123"
	longURL := "https://example.com"

	mockURLRepo.On("FindByShortCode", mock.Anything, shortCode).Return(&domain.URL{
		ID:        42,
		LongURL:   longURL,
		ShortCode: &shortCode,
	}, nil)
	mockCacheRepo.On("Get", mock.Anything, shortCode).Return("", errors.New("cache miss"))
	mockCacheRepo.On("Set", mock.Anything, shortCode, longURL).Return(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.ResolveURL(context.Background(), shortCode)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSqidsEncode measures the performance of sqids code generation directly.
func BenchmarkSqidsEncode(b *testing.B) {
	s, err := sqids.New(sqids.Options{MinLength: 6})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := s.Encode([]uint64{uint64(i)})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSqidsDecode measures the performance of sqids code decoding.
func BenchmarkSqidsDecode(b *testing.B) {
	s, err := sqids.New(sqids.Options{MinLength: 6})
	if err != nil {
		b.Fatal(err)
	}

	code, err := s.Encode([]uint64{42})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = s.Decode(code)
	}
}

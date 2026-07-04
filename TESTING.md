# Testing Guide for Deeper OSINT Tool

## Quick Start

### Run All Tests
```bash
make test
```

### Run Tests with Coverage
```bash
make test-coverage
# Opens coverage.html in browser
```

### Run Tests with Race Detector
```bash
make test-race
```

### Run Benchmarks
```bash
make benchmark
```

## Testing Patterns

The project uses **table-driven tests** as the primary testing pattern, following Go best practices.

### Example: Table-Driven Test

```go
func TestYourFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "test@example.com",
            expected: "email",
            wantErr:  false,
        },
        {
            name:     "invalid input",
            input:    "invalid",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := YourFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("YourFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if result != tt.expected {
                t.Errorf("YourFunction() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

## Testing Components

### 1. Testing Database Components

**Pattern**: Use temporary directories for test databases

```go
func TestDatabaseOperation(t *testing.T) {
    // Create temporary database
    tempDir := t.TempDir()
    dbPath := filepath.Join(tempDir, "test.db")

    db, err := NewDatabase(dbPath)
    require.NoError(t, err)
    defer db.Close()

    // Your test code here
}
```

**Example**: See `internal/pkg/database/database_test.go`

### 2. Testing Cache Components

**Pattern**: Test cache operations with various TTL scenarios

```go
func TestCache_SetAndGet(t *testing.T) {
    // Setup
    tempDir := t.TempDir()
    dbPath := filepath.Join(tempDir, "test.db")
    db, _ := NewDatabase(dbPath)
    defer db.Close()

    repo := NewRepository(db)
    cache := NewCache(repo)

    // Test Set
    trace := entities.Trace{Value: "test@example.com", Type: entities.Email}
    results := []entities.Trace{{Value: "result@example.com", Type: entities.Email}}
    err := cache.Set(trace, "TestPlugin", results, time.Hour)
    require.NoError(t, err)

    // Test Get
    retrieved, err := cache.Get(trace, "TestPlugin")
    require.NoError(t, err)
    assert.Equal(t, results, retrieved)
}
```

**Example**: See `internal/pkg/database/cache_test.go`

### 3. Testing Worker Pool

**Pattern**: Test concurrency, rate limiting, circuit breakers

```go
func TestWorkerPool_ConcurrentProcessing(t *testing.T) {
    config := &Config{
        MaxWorkers:       5,
        QueueSize:        100,
        DefaultRateLimit: rate.Limit(1000),
        DefaultBurst:     100,
        TaskTimeout:      5 * time.Second,
    }

    wp := NewWorkerPool(config)
    defer wp.Shutdown(10 * time.Second)

    // Submit concurrent tasks
    // Verify results
}
```

**Example**: See `internal/pkg/workerpool/workerpool_test.go`

### 4. Testing Entities

**Pattern**: Test trace type detection and validation

```go
func TestNewTrace(t *testing.T) {
    tests := []struct {
        name     string
        value    string
        expected TraceType
    }{
        {"email", "test@example.com", Email},
        {"domain", "example.com", Domain},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            trace := NewTrace(tt.value)
            assert.Equal(t, tt.expected, trace.Type)
        })
    }
}
```

**Example**: See `internal/pkg/entities/entities_test.go`

### 5. Testing Plugins

**Pattern**: Mock external dependencies, test trace processing

```go
func TestPlugin_FollowTrace(t *testing.T) {
    plugin := NewPlugin()

    trace := entities.Trace{
        Value: "test@example.com",
        Type:  entities.Email,
    }

    results, err := plugin.FollowTrace(trace)
    require.NoError(t, err)
    assert.NotEmpty(t, results)

    // Verify result types
    for _, result := range results {
        assert.NotEmpty(t, result.Value)
        assert.NotEmpty(t, result.Type)
    }
}
```

## Integration Testing

### Running Integration Tests

Integration tests are marked and can be skipped with `-short`:

```bash
# Skip integration tests
make test-short

# Run all tests including integration
make test
```

### Writing Integration Tests

```go
func TestIntegration_EndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup real components
    // Test complete workflow
    // Cleanup
}
```

**Example**: See `internal/pkg/workerpool/integration_test.go`

## Testing Best Practices

### 1. Use Testify

The project uses `testify` for assertions:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Use require for critical assertions (stops test on failure)
require.NoError(t, err)

// Use assert for non-critical assertions (continues test)
assert.Equal(t, expected, actual)
```

### 2. Test Isolation

- Each test should be independent
- Use `t.TempDir()` for temporary files
- Clean up resources with `defer`
- Don't rely on test execution order

### 3. Test Coverage Goals

- Aim for >80% test coverage
- Focus on critical paths
- Test error cases
- Test edge cases

### 4. Naming Conventions

- Test functions: `TestFunctionName` or `TestFunctionName_Scenario`
- Test files: `*_test.go`
- Test packages: Same package (not `*_test` package)

### 5. Table-Driven Tests

- Use for multiple test cases
- Include descriptive names
- Test both success and failure cases
- Test edge cases (empty, nil, boundary values)

## Running Specific Tests

### Run Tests in a Package
```bash
go test ./internal/pkg/database/...
```

### Run a Specific Test
```bash
go test -run TestCache_SetAndGet ./internal/pkg/database/
```

### Run Tests with Verbose Output
```bash
go test -v ./...
```

### Run Tests with Coverage for Specific Package
```bash
go test -coverprofile=coverage.out ./internal/pkg/database/
go tool cover -html=coverage.out
```

## Mocking External Dependencies

### HTTP Client Mocking

```go
type MockHTTPClient struct {
    GetFunc func(url string) (*http.Response, error)
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
    if m.GetFunc != nil {
        return m.GetFunc(url)
    }
    return nil, fmt.Errorf("not implemented")
}

func TestPlugin_WithMockHTTP(t *testing.T) {
    mockClient := &MockHTTPClient{
        GetFunc: func(url string) (*http.Response, error) {
            // Return mock response
        },
    }

    plugin := NewPlugin()
    plugin.client = mockClient

    // Test plugin
}
```

## Performance Testing

### Benchmarks

```go
func BenchmarkCache_Set(b *testing.B) {
    // Setup
    cache := setupCache()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Set(trace, "Plugin", results, time.Hour)
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./internal/pkg/database/
```

## Testing Checklist

Before submitting code, ensure:

- [ ] All tests pass: `make test`
- [ ] No race conditions: `make test-race`
- [ ] Good coverage: `make test-coverage` (aim for >80%)
- [ ] Tests are isolated (no shared state)
- [ ] Error cases are tested
- [ ] Edge cases are tested
- [ ] Integration tests pass (if applicable)
- [ ] Benchmarks are updated (if performance-critical)

## Common Testing Scenarios

### Testing with Time

```go
func TestCache_Expiration(t *testing.T) {
    // Use short durations for tests
    err := cache.Set(trace, "Plugin", results, 100*time.Millisecond)
    require.NoError(t, err)

    // Wait for expiration
    time.Sleep(150 * time.Millisecond)

    // Verify expired
    result, err := cache.Get(trace, "Plugin")
    assert.Nil(t, result)
}
```

### Testing Concurrency

```go
func TestConcurrentAccess(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            // Test concurrent operation
        }(i)
    }
    wg.Wait()
}
```

### Testing Error Handling

```go
func TestErrorHandling(t *testing.T) {
    // Test invalid input
    _, err := function("invalid")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "expected error message")

    // Test nil input
    _, err = function(nil)
    assert.Error(t, err)
}
```

## Debugging Tests

### Run Single Test with Verbose Output
```bash
go test -v -run TestSpecificFunction ./path/to/package
```

### Print Debug Information
```go
t.Logf("Debug: value = %v", value)
```

### Use Delve Debugger
```bash
dlv test ./internal/pkg/database/ -- -test.run TestCache
```

## CI/CD Testing

Tests run automatically in CI/CD. Ensure:

1. Tests are deterministic (no flaky tests)
2. Tests don't depend on external services (or are marked as integration tests)
3. Tests complete in reasonable time
4. All tests pass before merging

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Go Testing Best Practices](https://golang.org/doc/effective_go#testing)

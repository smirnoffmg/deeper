package errors

import (
	"errors"
	"testing"
)

func TestNewValidationError(t *testing.T) {
	cause := errors.New("test cause")
	err := NewValidationError("validation failed", cause)

	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected error type to be %s, got %s", ErrorTypeValidation, err.Type)
	}

	if err.Message != "validation failed" {
		t.Errorf("Expected error message to be 'validation failed', got %s", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected error cause to match original cause")
	}
}

func TestNewNetworkError(t *testing.T) {
	cause := errors.New("network timeout")
	err := NewNetworkError("connection failed", cause)

	if err.Type != ErrorTypeNetwork {
		t.Errorf("Expected error type to be %s, got %s", ErrorTypeNetwork, err.Type)
	}

	if err.Message != "connection failed" {
		t.Errorf("Expected error message to be 'connection failed', got %s", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected error cause to match original cause")
	}
}

func TestNewPluginError(t *testing.T) {
	cause := errors.New("plugin crash")
	err := NewPluginError("plugin execution failed", cause)

	if err.Type != ErrorTypePlugin {
		t.Errorf("Expected error type to be %s, got %s", ErrorTypePlugin, err.Type)
	}

	if err.Message != "plugin execution failed" {
		t.Errorf("Expected error message to be 'plugin execution failed', got %s", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected error cause to match original cause")
	}
}

func TestErrorString(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewValidationError("test error", cause)

	expected := "validation: test error (caused by: underlying error)"
	if err.Error() != expected {
		t.Errorf("Expected error string to be '%s', got '%s'", expected, err.Error())
	}
}

func TestErrorStringWithoutCause(t *testing.T) {
	err := NewValidationError("test error", nil)

	expected := "validation: test error"
	if err.Error() != expected {
		t.Errorf("Expected error string to be '%s', got '%s'", expected, err.Error())
	}
}

func TestWithContext(t *testing.T) {
	err := NewValidationError("test error", nil)
	err = err.WithContext("key1", "value1")
	err = err.WithContext("key2", 42)

	if err.Context["key1"] != "value1" {
		t.Errorf("Expected context key1 to be 'value1', got %v", err.Context["key1"])
	}

	if err.Context["key2"] != 42 {
		t.Errorf("Expected context key2 to be 42, got %v", err.Context["key2"])
	}
}

func TestIsValidationError(t *testing.T) {
	err := NewValidationError("test", nil)

	if !IsValidationError(err) {
		t.Error("Expected IsValidationError to return true for validation error")
	}

	if IsValidationError(errors.New("regular error")) {
		t.Error("Expected IsValidationError to return false for regular error")
	}
}

func TestIsNetworkError(t *testing.T) {
	err := NewNetworkError("test", nil)

	if !IsNetworkError(err) {
		t.Error("Expected IsNetworkError to return true for network error")
	}

	if IsNetworkError(errors.New("regular error")) {
		t.Error("Expected IsNetworkError to return false for regular error")
	}
}

func TestIsPluginError(t *testing.T) {
	err := NewPluginError("test", nil)

	if !IsPluginError(err) {
		t.Error("Expected IsPluginError to return true for plugin error")
	}

	if IsPluginError(errors.New("regular error")) {
		t.Error("Expected IsPluginError to return false for regular error")
	}
}

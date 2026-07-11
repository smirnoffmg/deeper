package errors

import (
	"errors"
	"testing"
)

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
	err := NewNetworkError("test error", cause)

	expected := "network: test error (caused by: underlying error)"
	if err.Error() != expected {
		t.Errorf("Expected error string to be '%s', got '%s'", expected, err.Error())
	}
}

func TestErrorStringWithoutCause(t *testing.T) {
	err := NewNetworkError("test error", nil)

	expected := "network: test error"
	if err.Error() != expected {
		t.Errorf("Expected error string to be '%s', got '%s'", expected, err.Error())
	}
}

func TestWithContext(t *testing.T) {
	err := NewNetworkError("test error", nil)
	err = err.WithContext("key1", "value1")
	err = err.WithContext("key2", 42)

	if err.Context["key1"] != "value1" {
		t.Errorf("Expected context key1 to be 'value1', got %v", err.Context["key1"])
	}

	if err.Context["key2"] != 42 {
		t.Errorf("Expected context key2 to be 42, got %v", err.Context["key2"])
	}
}

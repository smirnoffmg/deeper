package errors

import (
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeNetwork ErrorType = "network"
	ErrorTypePlugin  ErrorType = "plugin"
)

// DeeperError represents a structured error in the application
type DeeperError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *DeeperError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *DeeperError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *DeeperError) WithContext(key string, value interface{}) *DeeperError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, cause error) *DeeperError {
	return &DeeperError{
		Type:    ErrorTypeNetwork,
		Message: message,
		Cause:   cause,
	}
}

// NewPluginError creates a new plugin error
func NewPluginError(message string, cause error) *DeeperError {
	return &DeeperError{
		Type:    ErrorTypePlugin,
		Message: message,
		Cause:   cause,
	}
}

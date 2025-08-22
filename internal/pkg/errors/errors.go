package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeNetwork       ErrorType = "network"
	ErrorTypePlugin        ErrorType = "plugin"
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeInternal      ErrorType = "internal"
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

// NewValidationError creates a new validation error
func NewValidationError(message string, cause error) *DeeperError {
	return &DeeperError{
		Type:    ErrorTypeValidation,
		Message: message,
		Cause:   cause,
	}
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

// NewConfigurationError creates a new configuration error
func NewConfigurationError(message string, cause error) *DeeperError {
	return &DeeperError{
		Type:    ErrorTypeConfiguration,
		Message: message,
		Cause:   cause,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, cause error) *DeeperError {
	return &DeeperError{
		Type:    ErrorTypeInternal,
		Message: message,
		Cause:   cause,
	}
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if err != nil && err.Error() != "" {
		return strings.Contains(err.Error(), string(ErrorTypeValidation))
	}
	return false
}

// IsNetworkError checks if an error is a network error
func IsNetworkError(err error) bool {
	if err != nil && err.Error() != "" {
		return strings.Contains(err.Error(), string(ErrorTypeNetwork))
	}
	return false
}

// IsPluginError checks if an error is a plugin error
func IsPluginError(err error) bool {
	if err != nil && err.Error() != "" {
		return strings.Contains(err.Error(), string(ErrorTypePlugin))
	}
	return false
}

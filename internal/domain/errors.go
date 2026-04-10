package domain

import "fmt"

// SensimulError is the base error type for all Sensimul errors.
// It provides a hierarchy of error categories for proper exit code mapping.
type SensimulError struct {
	Code    ErrorCode
	Message string
	Err     error
}

type ErrorCode int

const (
	ErrCodeConfig ErrorCode = iota + 1
	ErrCodeValidation
	ErrCodeExternal
	ErrCodeRuntime
)

func (e *SensimulError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *SensimulError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a configuration-related error (exit code 2).
func NewConfigError(msg string, err error) *SensimulError {
	return &SensimulError{Code: ErrCodeConfig, Message: msg, Err: err}
}

// NewValidationError creates a validation error (exit code 2).
func NewValidationError(msg string) *SensimulError {
	return &SensimulError{Code: ErrCodeValidation, Message: msg}
}

// NewExternalError creates an external dependency error (exit code 3).
func NewExternalError(msg string, err error) *SensimulError {
	return &SensimulError{Code: ErrCodeExternal, Message: msg, Err: err}
}

// NewRuntimeError creates a runtime error (exit code 1).
func NewRuntimeError(msg string, err error) *SensimulError {
	return &SensimulError{Code: ErrCodeRuntime, Message: msg, Err: err}
}

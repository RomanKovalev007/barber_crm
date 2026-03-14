package apperr

import (
	"google.golang.org/grpc/codes"
)

type Code int

const (
	CodeInvalidArgument  Code = iota
	CodeNotFound
	CodeAlreadyExists
	CodeFailedPrecondition
	CodeInternal
)

type AppError struct {
	Code    Code
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) GRPCCode() codes.Code {
	switch e.Code {
	case CodeInvalidArgument:
		return codes.InvalidArgument
	case CodeNotFound:
		return codes.NotFound
	case CodeAlreadyExists:
		return codes.AlreadyExists
	case CodeFailedPrecondition:
		return codes.FailedPrecondition
	default:
		return codes.Internal
	}
}

func InvalidArgument(msg string) *AppError {
	return &AppError{Code: CodeInvalidArgument, Message: msg}
}

func NotFound(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

func AlreadyExists(msg string) *AppError {
	return &AppError{Code: CodeAlreadyExists, Message: msg}
}

func FailedPrecondition(msg string) *AppError {
	return &AppError{Code: CodeFailedPrecondition, Message: msg}
}

func Internal(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

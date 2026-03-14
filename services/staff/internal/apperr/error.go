package apperr

import (
	"google.golang.org/grpc/codes"
)

type Code int

const (
	CodeInvalidArgument Code = iota
	CodeNotFound
	CodeUnauthenticated
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
	case CodeUnauthenticated:
		return codes.Unauthenticated
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

func Unauthenticated(msg string) *AppError {
	return &AppError{Code: CodeUnauthenticated, Message: msg}
}

func Internal(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

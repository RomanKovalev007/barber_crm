package apperr

import "google.golang.org/grpc/codes"

type Code int

const (
	CodeInvalidArgument Code = iota
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
	default:
		return codes.Internal
	}
}

func InvalidArgument(msg string) *AppError {
	return &AppError{Code: CodeInvalidArgument, Message: msg}
}

func Internal(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

package response

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func WriteJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

type Error struct{
	Code string `json:"code"`
	Message string `json:"message"`
}

func ErrorJSON(w http.ResponseWriter, statusCode int, code, msg string){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(
		Error{
			Code: code,
			Message: msg,
		},
	)
}

// clientFacingCodes — коды, сообщение которых безопасно отдавать клиенту:
// они описывают ошибку запроса, а не внутреннее состояние сервиса.
var clientFacingCodes = map[codes.Code]bool{
	codes.InvalidArgument:   true,
	codes.NotFound:          true,
	codes.AlreadyExists:     true,
	codes.PermissionDenied:  true,
	codes.Unauthenticated:   true,
	codes.ResourceExhausted: true,
	codes.FailedPrecondition: true,
}

func GrpcErrorToHttp(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	httpCode := GrpcCodeToHTTP(st.Code())

	msg := "internal server error"
	if clientFacingCodes[st.Code()] {
		msg = st.Message()
	}

	ErrorJSON(w, httpCode, st.Code().String(), msg)
}

func GrpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
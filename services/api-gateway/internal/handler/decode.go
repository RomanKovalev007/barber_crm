package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/RomanKovalev007/barber_crm/services/api-gateway/pkg/response"
)

// decodeBody декодирует JSON-тело запроса в v.
// При превышении лимита тела отвечает 413, при прочих ошибках — 400.
// Возвращает false если произошла ошибка (ответ уже записан).
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.ErrorJSON(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body too large")
		} else {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		}
		return false
	}
	return true
}

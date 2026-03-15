package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/RomanKovalev007/barber_crm/pkg/auth"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/pkg/response"
)

type contextKey string

const barberIDKey contextKey = "barber_id"

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.ValidateToken(tokenStr, jwtSecret)
			if err != nil {
				response.ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), barberIDKey, claims.BarberID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func BarberIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(barberIDKey).(string)
	return v
}

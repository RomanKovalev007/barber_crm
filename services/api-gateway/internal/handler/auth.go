package handler

import (
	"net/http"

	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/pkg/response"
)

type AuthHandler struct {
	staff staffv1.StaffServiceClient
}

func NewAuthHandler(staff staffv1.StaffServiceClient) *AuthHandler {
	return &AuthHandler{staff: staff}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Login == "" || req.Password == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "login and password are required")
		return
	}

	resp, err := h.staff.Login(r.Context(), &staffv1.LoginRequest{
		Login:    req.Login,
		Password: req.Password,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
		"barber":        barberToModel(resp.Barber),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "refresh_token is required")
		return
	}

	_, err := h.staff.Logout(r.Context(), &staffv1.LogoutRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "refresh_token is required")
		return
	}

	resp, err := h.staff.RefreshToken(r.Context(), &staffv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
	})
}

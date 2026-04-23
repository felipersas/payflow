package http

import (
	"errors"
	"net/http"

	"github.com/felipersas/payflow/internal/user/application/commands"
	"github.com/felipersas/payflow/internal/user/application/services"
	"github.com/felipersas/payflow/pkg/httputil"
	"github.com/felipersas/payflow/pkg/validation"
	"github.com/go-chi/chi/v5"
)

// AuthHandler expõe endpoints de autenticação: register e login.
type AuthHandler struct {
	service *services.AuthService
}

func NewAuthHandler(service *services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Routes(r chi.Router) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	result, err := h.service.Register(r.Context(), commands.RegisterCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	result, err := h.service.Login(r.Context(), commands.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

// writeHandlerError distinguishes decode errors (400) from validation errors (422).
func writeHandlerError(w http.ResponseWriter, err error) {
	if httputil.IsDecodeError(err) {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var valErr *validation.ValidationError
	if errors.As(err, &valErr) {
		httputil.WriteValidationError(w, err)
		return
	}
	httputil.WriteError(w, err)
}

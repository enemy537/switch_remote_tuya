package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"switchremote/back/internal/auth"
	"switchremote/back/internal/store"
)

type meResponse struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if !readJSON(w, r, &req) {
		return
	}
	u, err := a.store.UserByEmail(r.Context(), normalizeEmail(req.Email))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		fail(w, err)
		return
	}
	if err != nil || !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}
	token, err := auth.IssueToken(a.jwtSecret, auth.Session{UserID: u.ID, IsAdmin: u.IsAdmin})
	if err != nil {
		fail(w, err)
		return
	}
	auth.SetCookie(w, token)
	writeJSON(w, http.StatusOK, meResponse{u.ID, u.Email, u.IsAdmin})
}

func (a *api) logout(w http.ResponseWriter, _ *http.Request) {
	auth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.store.UserByID(r.Context(), auth.CurrentSession(r.Context()).UserID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{u.ID, u.Email, u.IsAdmin})
}

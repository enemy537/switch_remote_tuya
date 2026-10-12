package httpapi

import (
	"net/http"
	"strconv"

	"switchremote/back/internal/auth"
	"switchremote/back/internal/store"
	"switchremote/back/internal/tuya"
)

// adminPageSize is one Tuya batch, so a page costs a single Tuya call.
const adminPageSize = tuya.MaxBatch

type userResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	IsAdmin     bool   `json:"is_admin"`
	TuyaUID     string `json:"tuya_uid"`
	DeviceCount int    `json:"device_count"`
}

type adminDeviceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserEmail string `json:"user_email"`
	Online    bool   `json:"online"`
	On        bool   `json:"on"`
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.Users(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = userResponse{u.ID, u.Email, u.IsAdmin, u.TuyaUID, u.DeviceCount}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TuyaUID  string `json:"tuya_uid"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	email := normalizeEmail(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	hash, ok := hashNewPassword(w, req.Password)
	if !ok {
		return
	}
	u := store.User{Email: email, PasswordHash: hash, IsAdmin: req.IsAdmin, TuyaUID: req.TuyaUID}
	id, err := a.store.CreateUser(r.Context(), u)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userResponse{id, u.Email, u.IsAdmin, u.TuyaUID, 0})
}

func (a *api) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req struct{ Password string }
	if !readJSON(w, r, &req) {
		return
	}
	hash, ok := hashNewPassword(w, req.Password)
	if !ok {
		return
	}
	if err := a.store.SetPassword(r.Context(), id, hash); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hashNewPassword validates and hashes a password, answering the request when it cannot.
func hashNewPassword(w http.ResponseWriter, password string) (string, bool) {
	if len(password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "password must have at least 8 characters")
		return "", false
	}
	hash, err := auth.HashPassword(password)
	if err != nil { // bcrypt rejects passwords longer than 72 bytes
		writeError(w, http.StatusBadRequest, "password is too long")
		return "", false
	}
	return hash, true
}

func (a *api) listAllDevices(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	devices, total, err := a.store.AllDevices(r.Context(), adminPageSize, (page-1)*adminPageSize)
	if err != nil {
		fail(w, err)
		return
	}
	ids := make([]string, len(devices))
	for i, d := range devices {
		ids[i] = d.ID
	}
	details, err := a.tuya.Devices(r.Context(), ids)
	if err != nil {
		fail(w, tuyaFailure(err))
		return
	}
	live := byID(details)
	items := make([]adminDeviceResponse, len(devices))
	for i, d := range devices {
		items[i] = adminDeviceResponse{d.ID, d.Name, d.UserEmail, live[d.ID].Online, live[d.ID].On}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page_size": adminPageSize})
}

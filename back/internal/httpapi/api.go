// Package httpapi exposes the JSON API under /api.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"switchremote/back/internal/auth"
	"switchremote/back/internal/store"
	"switchremote/back/internal/tuya"
)

const (
	maxBodyBytes      = 1 << 16
	minPasswordLength = 8
)

type Store interface {
	UserByEmail(ctx context.Context, email string) (store.User, error)
	UserByID(ctx context.Context, id int64) (store.User, error)
	Users(ctx context.Context) ([]store.User, error)
	CreateUser(ctx context.Context, u store.User) (int64, error)
	SetPassword(ctx context.Context, userID int64, hash string) error
	DevicesByUser(ctx context.Context, userID int64) ([]store.Device, error)
	RegisteredIDs(ctx context.Context, ids []string) ([]string, error)
	CreateDevice(ctx context.Context, d store.Device) error
	DeleteDevice(ctx context.Context, id string, userID int64) error
	OwnsDevice(ctx context.Context, id string, userID int64) (bool, error)
	AllDevices(ctx context.Context, limit, offset int) ([]store.Device, int, error)
}

type Tuya interface {
	UserDevices(ctx context.Context, uid string) ([]tuya.Device, error)
	Devices(ctx context.Context, ids []string) ([]tuya.Device, error)
	SetSwitch(ctx context.Context, deviceID string, on bool) error
}

type api struct {
	store     Store
	tuya      Tuya
	jwtSecret []byte
}

func New(s Store, t Tuya, jwtSecret []byte) http.Handler {
	a := &api{store: s, tuya: t, jwtSecret: jwtSecret}
	user := func(h http.HandlerFunc) http.HandlerFunc { return auth.RequireUser(jwtSecret, h) }
	admin := func(h http.HandlerFunc) http.HandlerFunc { return auth.RequireAdmin(jwtSecret, h) }

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", a.logout)
	mux.HandleFunc("GET /api/me", user(a.me))
	mux.HandleFunc("GET /api/tuya-devices", user(a.listTuyaDevices))
	mux.HandleFunc("GET /api/devices", user(a.listDevices))
	mux.HandleFunc("POST /api/devices", user(a.registerDevice))
	mux.HandleFunc("DELETE /api/devices/{id}", user(a.deleteDevice))
	mux.HandleFunc("POST /api/devices/{id}/switch", user(a.switchDevice))
	mux.HandleFunc("GET /api/admin/users", admin(a.listUsers))
	mux.HandleFunc("POST /api/admin/users", admin(a.createUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}", admin(a.resetPassword))
	mux.HandleFunc("GET /api/admin/devices", admin(a.listAllDevices))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes the request body into v, answering 400 when it cannot.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// errTuya marks failures of the Tuya cloud, which are answered with 502.
var errTuya = errors.New("tuya")

func tuyaFailure(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errTuya, err)
}

// fail answers with the status that matches err.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "already exists")
	case errors.Is(err, errTuya):
		log.Print(err)
		writeError(w, http.StatusBadGateway, "could not reach Tuya, try again")
	default:
		log.Print(err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

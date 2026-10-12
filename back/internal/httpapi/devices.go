package httpapi

import (
	"context"
	"net/http"
	"slices"

	"switchremote/back/internal/auth"
	"switchremote/back/internal/store"
	"switchremote/back/internal/tuya"
)

type deviceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Online bool   `json:"online"`
	On     bool   `json:"on"`
}

type tuyaDeviceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Online bool   `json:"online"`
}

// accountDevices lists the devices in the caller's Smart Life account.
// A user whose account is not linked yet has none.
func (a *api) accountDevices(ctx context.Context) ([]tuya.Device, error) {
	u, err := a.store.UserByID(ctx, auth.CurrentSession(ctx).UserID)
	if err != nil || u.TuyaUID == "" {
		return nil, err
	}
	devices, err := a.tuya.UserDevices(ctx, u.TuyaUID)
	return devices, tuyaFailure(err)
}

func byID(devices []tuya.Device) map[string]tuya.Device {
	m := make(map[string]tuya.Device, len(devices))
	for _, d := range devices {
		m[d.ID] = d
	}
	return m
}

func (a *api) listTuyaDevices(w http.ResponseWriter, r *http.Request) {
	account, err := a.accountDevices(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	ids := make([]string, len(account))
	for i, d := range account {
		ids[i] = d.ID
	}
	registered, err := a.store.RegisteredIDs(r.Context(), ids)
	if err != nil {
		fail(w, err)
		return
	}
	out := []tuyaDeviceResponse{}
	for _, d := range account {
		if !slices.Contains(registered, d.ID) {
			out = append(out, tuyaDeviceResponse{d.ID, d.Name, d.Online})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) listDevices(w http.ResponseWriter, r *http.Request) {
	registered, err := a.store.DevicesByUser(r.Context(), auth.CurrentSession(r.Context()).UserID)
	if err != nil {
		fail(w, err)
		return
	}
	account, err := a.accountDevices(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	live := byID(account)
	out := make([]deviceResponse, len(registered))
	for i, d := range registered {
		// A device removed from the Smart Life account shows as offline.
		out[i] = deviceResponse{d.ID, d.Name, live[d.ID].Online, live[d.ID].On}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req struct{ ID, Name string }
	if !readJSON(w, r, &req) {
		return
	}
	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "id and name are required")
		return
	}
	account, err := a.accountDevices(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	live, ok := byID(account)[req.ID]
	if !ok {
		writeError(w, http.StatusForbidden, "device is not in your Smart Life account")
		return
	}
	d := store.Device{ID: req.ID, UserID: auth.CurrentSession(r.Context()).UserID, Name: req.Name}
	if err := a.store.CreateDevice(r.Context(), d); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, deviceResponse{d.ID, d.Name, live.Online, live.On})
}

func (a *api) deleteDevice(w http.ResponseWriter, r *http.Request) {
	err := a.store.DeleteDevice(r.Context(), r.PathValue("id"), auth.CurrentSession(r.Context()).UserID)
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) switchDevice(w http.ResponseWriter, r *http.Request) {
	var req struct{ On *bool }
	if !readJSON(w, r, &req) {
		return
	}
	if req.On == nil {
		writeError(w, http.StatusBadRequest, "on is required")
		return
	}
	id := r.PathValue("id")
	owns, err := a.store.OwnsDevice(r.Context(), id, auth.CurrentSession(r.Context()).UserID)
	if err == nil && !owns {
		err = store.ErrNotFound
	}
	if err == nil {
		err = tuyaFailure(a.tuya.SetSwitch(r.Context(), id, *req.On))
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

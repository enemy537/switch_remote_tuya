package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"switchremote/back/internal/auth"
	"switchremote/back/internal/store"
	"switchremote/back/internal/tuya"
)

var secret = []byte("test-secret")

// Fixture: alice (admin, id 1) owns device "a"; bob (id 2) owns nothing.
// Alice's Smart Life account holds "a" and "c"; bob's holds "b".
type fakeStore struct {
	Store   // unused methods panic
	users   []store.User
	devices []store.Device
}

func (f *fakeStore) UserByEmail(_ context.Context, email string) (store.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeStore) UserByID(_ context.Context, id int64) (store.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeStore) Users(context.Context) ([]store.User, error) { return f.users, nil }

func (f *fakeStore) DevicesByUser(_ context.Context, userID int64) ([]store.Device, error) {
	var out []store.Device
	for _, d := range f.devices {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeStore) RegisteredIDs(context.Context, []string) ([]string, error) {
	var ids []string
	for _, d := range f.devices {
		ids = append(ids, d.ID)
	}
	return ids, nil
}

func (f *fakeStore) CreateDevice(_ context.Context, d store.Device) error {
	for _, existing := range f.devices {
		if existing.ID == d.ID {
			return store.ErrConflict
		}
	}
	f.devices = append(f.devices, d)
	return nil
}

func (f *fakeStore) OwnsDevice(_ context.Context, id string, userID int64) (bool, error) {
	for _, d := range f.devices {
		if d.ID == id && d.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

type fakeTuya struct {
	accounts map[string][]tuya.Device
	switched []string
	err      error
}

func (f *fakeTuya) UserDevices(_ context.Context, uid string) ([]tuya.Device, error) {
	return f.accounts[uid], f.err
}

func (f *fakeTuya) Devices(context.Context, []string) ([]tuya.Device, error) { return nil, f.err }

func (f *fakeTuya) SetSwitch(_ context.Context, id string, _ bool) error {
	f.switched = append(f.switched, id)
	return f.err
}

type fixture struct {
	handler http.Handler
	store   *fakeStore
	tuya    *fakeTuya
}

func newFixture(t *testing.T) *fixture {
	hash, err := auth.HashPassword("alice-password")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeStore{
		users: []store.User{
			{ID: 1, Email: "alice@example.com", PasswordHash: hash, IsAdmin: true, TuyaUID: "uid-alice"},
			{ID: 2, Email: "bob@example.com", TuyaUID: "uid-bob"},
		},
		devices: []store.Device{{ID: "a", UserID: 1, Name: "Porch"}},
	}
	tu := &fakeTuya{accounts: map[string][]tuya.Device{
		"uid-alice": {{ID: "a", Name: "Lamp", Online: true, On: true}, {ID: "c", Name: "Heater", Online: true}},
		"uid-bob":   {{ID: "b", Name: "Fan"}},
	}}
	return &fixture{New(s, tu, secret), s, tu}
}

// do performs a request, logged in as userID unless it is 0.
func (f *fixture) do(t *testing.T, userID int64, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != 0 {
		token, err := auth.IssueToken(secret, auth.Session{UserID: userID, IsAdmin: userID == 1})
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "session", Value: token})
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != status || (body != "" && got != body) {
		t.Errorf("got %d %s; want %d %s", rec.Code, got, status, body)
	}
}

func TestLogin(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, 0, "POST", "/api/login", `{"email":" Alice@Example.com","password":"alice-password"}`)
	expect(t, rec, 200, `{"id":1,"email":"alice@example.com","is_admin":true}`)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}
	c := cookies[0]
	if c.Name != "session" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 86400 {
		t.Errorf("cookie = %+v", c)
	}
	if s, err := auth.VerifyToken(secret, c.Value); err != nil || s != (auth.Session{UserID: 1, IsAdmin: true}) {
		t.Errorf("session = %+v, %v", s, err)
	}

	expect(t, f.do(t, 0, "POST", "/api/login", `{"email":"alice@example.com","password":"wrong"}`), 401, "")
	expect(t, f.do(t, 0, "POST", "/api/login", `{"email":"nobody@example.com","password":"alice-password"}`), 401, "")
}

func TestAuthRequired(t *testing.T) {
	f := newFixture(t)
	for _, route := range []string{"GET /api/me", "GET /api/devices", "GET /api/tuya-devices", "POST /api/devices", "GET /api/admin/users"} {
		method, path, _ := strings.Cut(route, " ")
		expect(t, f.do(t, 0, method, path, ""), 401, `{"error":"not logged in"}`)
	}
	expect(t, f.do(t, 1, "GET", "/api/me", ""), 200, `{"id":1,"email":"alice@example.com","is_admin":true}`)
}

func TestAdminRequired(t *testing.T) {
	f := newFixture(t)
	expect(t, f.do(t, 2, "GET", "/api/admin/users", ""), 403, `{"error":"admin only"}`)
	expect(t, f.do(t, 2, "POST", "/api/admin/users", `{"email":"x@example.com","password":"12345678"}`), 403, "")
	expect(t, f.do(t, 1, "GET", "/api/admin/users", ""), 200, "")
}

func TestListDevices(t *testing.T) {
	f := newFixture(t)
	expect(t, f.do(t, 1, "GET", "/api/devices", ""), 200, `[{"id":"a","name":"Porch","online":true,"on":true}]`)
	expect(t, f.do(t, 1, "GET", "/api/tuya-devices", ""), 200, `[{"id":"c","name":"Heater","online":true}]`)
	expect(t, f.do(t, 2, "GET", "/api/devices", ""), 200, `[]`)

	f.tuya.err = errors.New("connection refused")
	expect(t, f.do(t, 1, "GET", "/api/devices", ""), 502, `{"error":"could not reach Tuya, try again"}`)
}

func TestRegisterDevice(t *testing.T) {
	f := newFixture(t)
	// "c" is in alice's Smart Life account, not bob's.
	expect(t, f.do(t, 2, "POST", "/api/devices", `{"id":"c","name":"Mine now"}`), 403, "")
	if len(f.store.devices) != 1 {
		t.Fatalf("device was registered: %+v", f.store.devices)
	}
	expect(t, f.do(t, 1, "POST", "/api/devices", `{"id":"c","name":"Heater"}`), 201, `{"id":"c","name":"Heater","online":true,"on":false}`)
	expect(t, f.do(t, 1, "POST", "/api/devices", `{"id":"c","name":"Heater"}`), 409, "")
}

func TestSwitchDevice(t *testing.T) {
	f := newFixture(t)
	expect(t, f.do(t, 2, "POST", "/api/devices/a/switch", `{"on":true}`), 404, "")
	if len(f.tuya.switched) != 0 {
		t.Fatalf("someone else's device was switched: %v", f.tuya.switched)
	}
	expect(t, f.do(t, 1, "POST", "/api/devices/a/switch", `{}`), 400, "")
	expect(t, f.do(t, 1, "POST", "/api/devices/a/switch", `{"on":true}`), 204, "")
	if len(f.tuya.switched) != 1 || f.tuya.switched[0] != "a" {
		t.Errorf("switched = %v", f.tuya.switched)
	}
}

package tuya

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Vectors from the worked examples in Tuya's "Sign Requests for Cloud Authorization" page.
func TestSignMatchesDocumentedExamples(t *testing.T) {
	const (
		clientID = "1KAD46OrT9HafiKdsXeg"
		secret   = "4OHBOnWOqaEC1mWXOpVL3yV50s0qGSRC"
		ts       = "1588925778000"
		nonce    = "5138cc3a9033d69856923fd07b491173"
		headers  = "area_id:29a33e8796834b1efa6\ncall_id:8afdb70ab2ed11eb85290242ac130003\n"
	)
	tokenRequest := stringToSign("GET", nil, headers, "/v1.0/token?grant_type=1")
	if got := sign(clientID, secret, "", ts, nonce, tokenRequest); got != "9E48A3E93B302EEECC803C7241985D0A34EB944F40FB573C7B5C2A82158AF13E" {
		t.Errorf("token request sign = %s", got)
	}
	business := stringToSign("GET", nil, headers, "/v2.0/apps/schema/users?page_no=1&page_size=50")
	if got := sign(clientID, secret, "3f4eda2bdec17232f67c0b188af3eec1", ts, nonce, business); got != "AE4481C692AA80B25F3A7E12C3A5FD9BBF6251539DD78E565A1A72A508A88784" {
		t.Errorf("business request sign = %s", got)
	}
}

// fakeTuya verifies every signature the way the real service does and records the requests.
func fakeTuya(t *testing.T, requests *[]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		header := func(name string) string { return strings.Join(r.Header[http.CanonicalHeaderKey(name)], "") }
		want := sign("id", "secret", header("access_token"), header("t"), "",
			stringToSign(r.Method, body, "", r.URL.RequestURI()))
		if header("sign") != want || header("client_id") != "id" || header("sign_method") != "HMAC-SHA256" {
			t.Errorf("%s %s: bad signature headers %v", r.Method, r.URL, r.Header)
		}
		*requests = append(*requests, r.Method+" "+r.URL.RequestURI()+" "+string(body))

		if r.URL.Path == "/v1.0/token" {
			io.WriteString(w, `{"success":true,"result":{"access_token":"tok","expire_time":7200}}`)
			return
		}
		if header("access_token") != "tok" {
			t.Errorf("%s %s: access_token = %q", r.Method, r.URL, header("access_token"))
		}
		switch {
		case r.URL.Path == "/v1.0/users/uid1/devices":
			io.WriteString(w, `{"success":true,"result":[
				{"id":"a","name":"Lamp","online":true,"status":[{"code":"countdown_1","value":0},{"code":"switch_1","value":true}]},
				{"id":"b","name":"Fan","online":false,"status":[{"code":"switch_1","value":false}]}]}`)
		case r.URL.Path == "/v1.0/devices":
			io.WriteString(w, `{"success":true,"result":{"total":1,"devices":[{"id":"a","name":"Lamp","online":true,"status":[{"code":"switch_1","value":true}]}]}}`)
		case strings.HasSuffix(r.URL.Path, "/commands"):
			io.WriteString(w, `{"success":true,"result":true}`)
		default:
			io.WriteString(w, `{"success":false,"code":1108,"msg":"uri path invalid"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient(t *testing.T) {
	var requests []string
	c := New(fakeTuya(t, &requests).URL, "id", "secret")
	ctx := context.Background()

	devices, err := c.UserDevices(ctx, "uid1")
	if err != nil {
		t.Fatal(err)
	}
	want := []Device{{ID: "a", Name: "Lamp", Online: true, On: true}, {ID: "b", Name: "Fan"}}
	if len(devices) != 2 || devices[0] != want[0] || devices[1] != want[1] {
		t.Errorf("UserDevices = %+v, want %+v", devices, want)
	}

	batch, err := c.Devices(ctx, []string{"a", "b"})
	if err != nil || len(batch) != 1 || batch[0] != want[0] {
		t.Errorf("Devices = %+v, %v", batch, err)
	}

	if err := c.SetSwitch(ctx, "a", true); err != nil {
		t.Fatal(err)
	}

	wantRequests := []string{
		"GET /v1.0/token?grant_type=1 ", // fetched once, then cached
		"GET /v1.0/users/uid1/devices ",
		"GET /v1.0/devices?device_ids=a,b&page_no=1&page_size=20 ",
		`POST /v1.0/devices/a/commands {"commands":[{"code":"switch_1","value":true}]}`,
	}
	if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(requests, "\n"), strings.Join(wantRequests, "\n"))
	}
}

func TestFailureIncludesCodeAndMessage(t *testing.T) {
	var requests []string
	c := New(fakeTuya(t, &requests).URL, "id", "secret")
	err := c.call(context.Background(), "GET", "/v1.0/nope", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "1108") || !strings.Contains(err.Error(), "uri path invalid") {
		t.Errorf("err = %v", err)
	}
}

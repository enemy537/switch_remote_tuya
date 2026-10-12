// Package tuya is a minimal signed client for the Tuya cloud OpenAPI.
package tuya

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// switchCode is the data point of a single-channel switch.
const switchCode = "switch_1"

// MaxBatch is how many devices Devices accepts per call.
const MaxBatch = 20

type Device struct {
	ID     string
	Name   string
	Online bool
	On     bool
}

func (d *Device) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Online bool   `json:"online"`
		Status []struct {
			Code  string `json:"code"`
			Value any    `json:"value"`
		} `json:"status"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*d = Device{ID: raw.ID, Name: raw.Name, Online: raw.Online}
	for _, s := range raw.Status {
		if s.Code == switchCode {
			d.On = s.Value == true
		}
	}
	return nil
}

type Client struct {
	baseURL, clientID, secret string
	http                      *http.Client

	mu           sync.Mutex
	token        string
	tokenExpires time.Time
}

func New(baseURL, clientID, secret string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		clientID: clientID,
		secret:   secret,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// UserDevices lists the devices in a Smart Life account.
func (c *Client) UserDevices(ctx context.Context, uid string) ([]Device, error) {
	var devices []Device
	err := c.call(ctx, http.MethodGet, "/v1.0/users/"+uid+"/devices", nil, &devices)
	return devices, err
}

// Devices returns the live details of up to MaxBatch devices.
func (c *Client) Devices(ctx context.Context, ids []string) ([]Device, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var result struct {
		Devices []Device `json:"devices"`
	}
	// Query keys must stay in alphabetical order: the signature covers them sorted.
	path := fmt.Sprintf("/v1.0/devices?device_ids=%s&page_no=1&page_size=%d", strings.Join(ids, ","), MaxBatch)
	err := c.call(ctx, http.MethodGet, path, nil, &result)
	return result.Devices, err
}

func (c *Client) SetSwitch(ctx context.Context, deviceID string, on bool) error {
	type command struct {
		Code  string `json:"code"`
		Value bool   `json:"value"`
	}
	body := map[string][]command{"commands": {{Code: switchCode, Value: on}}}
	return c.call(ctx, http.MethodPost, "/v1.0/devices/"+deviceID+"/commands", body, nil)
}

func (c *Client) call(ctx context.Context, method, path string, body, result any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	return c.send(ctx, method, path, token, body, result)
}

// accessToken returns the cached project token, fetching a new one shortly before it expires.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.tokenExpires) {
		return c.token, nil
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpireTime  int    `json:"expire_time"` // seconds
	}
	if err := c.send(ctx, http.MethodGet, "/v1.0/token?grant_type=1", "", nil, &result); err != nil {
		return "", err
	}
	c.token = result.AccessToken
	c.tokenExpires = time.Now().Add(time.Duration(result.ExpireTime)*time.Second - time.Minute)
	return c.token, nil
}

// send signs and performs one request. An empty token is used only by the token request itself.
func (c *Client) send(ctx context.Context, method, path, token string, body, result any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	t := strconv.FormatInt(time.Now().UnixMilli(), 10)
	// Assigned directly: Tuya's header names are lowercase with underscores.
	req.Header["client_id"] = []string{c.clientID}
	req.Header["t"] = []string{t}
	req.Header["sign_method"] = []string{"HMAC-SHA256"}
	req.Header["sign"] = []string{sign(c.clientID, c.secret, token, t, "", stringToSign(method, payload, "", path))}
	if token != "" {
		req.Header["access_token"] = []string{token}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tuya %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	var envelope struct {
		Success bool            `json:"success"`
		Code    int             `json:"code"`
		Msg     string          `json:"msg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("tuya %s %s: status %d: %w", method, path, resp.StatusCode, err)
	}
	if !envelope.Success {
		return fmt.Errorf("tuya %s %s: code %d: %s", method, path, envelope.Code, envelope.Msg)
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

// stringToSign builds the request summary Tuya signs. headers is the optional
// "name:value\n" block of extra signed headers; url is the path with its query sorted by key.
func stringToSign(method string, body []byte, headers, url string) string {
	sum := sha256.Sum256(body)
	return method + "\n" + hex.EncodeToString(sum[:]) + "\n" + headers + "\n" + url
}

// sign computes Tuya's HMAC-SHA256 signature. token is empty for the token request.
func sign(clientID, secret, token, t, nonce, stringToSign string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(clientID + token + t + nonce + stringToSign))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

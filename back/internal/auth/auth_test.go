package auth

import (
	"strings"
	"testing"
	"time"
)

var secret = []byte("test-secret")

func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse") {
		t.Error("right password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("wrong password accepted")
	}
	// The first admin is seeded with pgcrypto, which writes the same $2a$ format.
	if !strings.HasPrefix(hash, "$2a$10$") {
		t.Errorf("hash %q is not $2a$ bcrypt", hash)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	want := Session{UserID: 7, IsAdmin: true}
	token, err := IssueToken(secret, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyToken(secret, token)
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	token, _ := issueToken(secret, Session{UserID: 7}, time.Now().Add(-time.Minute))
	if _, err := VerifyToken(secret, token); err == nil {
		t.Error("expired token accepted")
	}
}

func TestForgedTokenRejected(t *testing.T) {
	token, _ := IssueToken([]byte("another-secret"), Session{UserID: 7, IsAdmin: true})
	if _, err := VerifyToken(secret, token); err == nil {
		t.Error("token signed with another secret accepted")
	}
}

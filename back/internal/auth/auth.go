// Package auth holds password hashing, the session JWT and the middleware that guards routes.
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName = "session"
	sessionTTL = 24 * time.Hour
)

// Session is who the caller is, as carried by the session cookie.
type Session struct {
	UserID  int64
	IsAdmin bool
}

type claims struct {
	UserID  int64 `json:"uid"`
	IsAdmin bool  `json:"adm"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func IssueToken(secret []byte, s Session) (string, error) {
	return issueToken(secret, s, time.Now().Add(sessionTTL))
}

func issueToken(secret []byte, s Session, expires time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID:           s.UserID,
		IsAdmin:          s.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expires)},
	}).SignedString(secret)
}

func VerifyToken(secret []byte, token string) (Session, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return Session{}, err
	}
	if c.UserID == 0 {
		return Session{}, errors.New("token has no user")
	}
	return Session{UserID: c.UserID, IsAdmin: c.IsAdmin}, nil
}

func SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, cookie(token, int(sessionTTL.Seconds())))
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, cookie("", -1))
}

func cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

type contextKey struct{}

// CurrentSession returns the session stored by RequireUser or RequireAdmin.
func CurrentSession(ctx context.Context) Session {
	s, _ := ctx.Value(contextKey{}).(Session)
	return s
}

// RequireUser answers 401 unless the request carries a valid session cookie.
func RequireUser(secret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			deny(w, http.StatusUnauthorized, "not logged in")
			return
		}
		s, err := VerifyToken(secret, c.Value)
		if err != nil {
			deny(w, http.StatusUnauthorized, "not logged in")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, s)))
	}
}

// RequireAdmin answers 401 without a session and 403 when the user is not an admin.
func RequireAdmin(secret []byte, next http.HandlerFunc) http.HandlerFunc {
	return RequireUser(secret, func(w http.ResponseWriter, r *http.Request) {
		if !CurrentSession(r.Context()).IsAdmin {
			deny(w, http.StatusForbidden, "admin only")
			return
		}
		next(w, r)
	})
}

func deny(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}

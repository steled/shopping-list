package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName      = "sl_session"
	sessionDuration = 7 * 24 * time.Hour
)

// Auth handles password validation and session cookies.
type Auth struct {
	username     string
	passwordHash []byte
	secret       []byte
}

type sessionPayload struct {
	Exp int64 `json:"exp"`
}

// New creates a new Auth instance. The password is hashed with bcrypt at
// startup so that login attempts always perform a constant-time compare.
func New(username, password, secret string) *Auth {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: bcrypt hash failed: " + err.Error())
	}
	return &Auth{
		username:     username,
		passwordHash: hash,
		secret:       []byte(secret),
	}
}

// Validate returns true when the provided credentials are correct.
func (a *Auth) Validate(username, password string) bool {
	if username != a.username {
		// Still run bcrypt to avoid timing side-channel.
		_ = bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)) == nil
}

// SetSessionCookie writes a signed, HttpOnly session cookie to the response.
func (a *Auth) SetSessionCookie(w http.ResponseWriter, r *http.Request) {
	payload := sessionPayload{Exp: time.Now().Add(sessionDuration).Unix()}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	value := payloadB64 + "." + a.sign(payloadB64)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

// ClearSessionCookie expires the session cookie.
func (a *Auth) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// IsAuthenticated returns true when the request carries a valid session cookie.
func (a *Auth) IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payloadB64, sig := parts[0], parts[1]

	// Constant-time signature comparison to prevent timing attacks.
	if !hmac.Equal([]byte(a.sign(payloadB64)), []byte(sig)) {
		return false
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return false
	}
	var payload sessionPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return false
	}
	return time.Now().Unix() < payload.Exp
}

func (a *Auth) sign(data string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

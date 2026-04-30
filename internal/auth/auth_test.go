package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSecret = "test-hmac-secret-for-unit-tests-padding"

func TestValidate(t *testing.T) {
	a := New("admin", "secret123", testSecret)

	if !a.Validate("admin", "secret123") {
		t.Error("expected valid credentials to pass")
	}
	if a.Validate("admin", "wrong") {
		t.Error("expected wrong password to fail")
	}
	if a.Validate("other", "secret123") {
		t.Error("expected wrong username to fail")
	}
}

func TestSessionCookie(t *testing.T) {
	a := New("admin", "password", testSecret)

	// Set cookie
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	a.SetSessionCookie(w, r)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}
	cookie := cookies[0]

	// Valid cookie authenticates
	r2 := httptest.NewRequest(http.MethodGet, "/list", nil)
	r2.AddCookie(cookie)
	if !a.IsAuthenticated(r2) {
		t.Error("expected request with valid cookie to be authenticated")
	}

	// Tampered value must fail
	r3 := httptest.NewRequest(http.MethodGet, "/list", nil)
	r3.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value + "x"})
	if a.IsAuthenticated(r3) {
		t.Error("expected tampered cookie to fail authentication")
	}

	// No cookie must fail
	r4 := httptest.NewRequest(http.MethodGet, "/list", nil)
	if a.IsAuthenticated(r4) {
		t.Error("expected request without cookie to fail authentication")
	}
}

func TestClearCookie(t *testing.T) {
	a := New("admin", "password", testSecret)

	w := httptest.NewRecorder()
	a.ClearSessionCookie(w)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a cookie in the clear response")
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("expected MaxAge=-1, got %d", cookies[0].MaxAge)
	}
}

func TestDifferentSecretDoesNotAuthenticate(t *testing.T) {
	a1 := New("admin", "password", "secret-one-padded-long-enough-!!!")
	a2 := New("admin", "password", "secret-two-padded-long-enough-!!!")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	a1.SetSessionCookie(w, r)

	cookies := w.Result().Cookies()
	r2 := httptest.NewRequest(http.MethodGet, "/list", nil)
	r2.AddCookie(cookies[0])

	if a2.IsAuthenticated(r2) {
		t.Error("expected cookie signed with different secret to fail")
	}
}

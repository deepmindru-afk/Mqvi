package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akinalp/mqvi/pkg/authcookie"
)

func TestReadJWTToken_Cookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	r.AddCookie(&http.Cookie{Name: authcookie.Name, Value: "cookie-jwt"})
	if got := readJWTToken(r); got != "cookie-jwt" {
		t.Fatalf("got %q want %q", got, "cookie-jwt")
	}
}

func TestReadJWTToken_BearerHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	r.Header.Set("Authorization", "Bearer header-jwt")
	if got := readJWTToken(r); got != "header-jwt" {
		t.Fatalf("got %q want %q", got, "header-jwt")
	}
}

// Cookie wins so browser users aren't overridden by an unexpected bearer header.
func TestReadJWTToken_CookieWinsOverBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	r.AddCookie(&http.Cookie{Name: authcookie.Name, Value: "cookie-jwt"})
	r.Header.Set("Authorization", "Bearer header-jwt")
	if got := readJWTToken(r); got != "cookie-jwt" {
		t.Fatalf("got %q want %q", got, "cookie-jwt")
	}
}

func TestReadJWTToken_NoAuthReturnsEmpty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	if got := readJWTToken(r); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestReadJWTToken_MalformedAuthorizationIgnored(t *testing.T) {
	cases := []string{
		"Basic abc:def",
		"bearer lowercase-prefix",
		"Bearer", // no token
		"BearerNoSpace",
	}
	for _, h := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
		r.Header.Set("Authorization", h)
		got := readJWTToken(r)
		if h == "Bearer" {
			// "Bearer" alone — strip leaves empty
			if got != "" {
				t.Errorf("Authorization=%q: got %q want empty", h, got)
			}
			continue
		}
		if got != "" {
			t.Errorf("Authorization=%q: got %q want empty (only %q-prefixed accepted)", h, got, "Bearer ")
		}
	}
}

func TestReadJWTToken_OtherCookiesIgnored(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "wrong-cookie"})
	r.AddCookie(&http.Cookie{Name: "csrf", Value: "noise"})
	if got := readJWTToken(r); got != "" {
		t.Fatalf("only mqvi_file_session counts; got %q", got)
	}
}

func TestReadJWTToken_BearerWithExtraSpacesPreserved(t *testing.T) {
	// Don't trim extra whitespace — downstream JWT validation rejects it.
	r := httptest.NewRequest(http.MethodGet, "/api/files/avatars/u/x.png", nil)
	r.Header.Set("Authorization", "Bearer  jwt")
	if got := readJWTToken(r); got != " jwt" {
		t.Fatalf("got %q want leading space preserved", got)
	}
}

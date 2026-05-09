// Package authcookie sets the /api/files session cookie used as a fallback
// when the signed URL TTL has elapsed.
package authcookie

import (
	"net/http"
	"time"
)

const Name = "mqvi_file_session"

// Path scopes the cookie so browsers don't replay it on /api/* mutations.
const Path = "/api/files"

// Set writes the cookie. SameSite=None+Secure is required for the Electron
// renderer (file:// → API is cross-site).
func Set(w http.ResponseWriter, accessToken string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     Name,
		Value:    accessToken,
		Path:     Path,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}

func Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     Name,
		Value:    "",
		Path:     Path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}

func Read(r *http.Request) string {
	c, err := r.Cookie(Name)
	if err != nil {
		return ""
	}
	return c.Value
}

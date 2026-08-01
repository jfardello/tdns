package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jfardello/tdns/internal/browserauth"
)

const (
	sessionCookieName = "__Host-tdns-session"
	csrfHeaderName    = "X-CSRF-Token"
)

var errCrossSiteRequest = errors.New("cross-site request rejected")

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validateBrowserOrigin(r *http.Request) error {
	switch site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); site {
	case "same-origin":
		return nil
	case "same-site", "cross-site":
		return errCrossSiteRequest
	case "none":
		return validateOriginOrReferer(r)
	case "":
		return validateOriginOrReferer(r)
	default:
		return errCrossSiteRequest
	}
}

func validateOriginOrReferer(r *http.Request) error {
	expected := "https://" + r.Host
	if origins := r.Header.Values("Origin"); len(origins) > 0 {
		if len(origins) != 1 || strings.Contains(origins[0], " ") || origins[0] != expected {
			return errCrossSiteRequest
		}
		return nil
	}
	referer := strings.TrimSpace(r.Referer())
	if referer == "" {
		return errCrossSiteRequest
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Host != r.Host {
		return errCrossSiteRequest
	}
	return nil
}

func setSessionCookie(w http.ResponseWriter, credentials browserauth.Credentials) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    credentials.SessionID,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	if credentials.Session.Persistent {
		cookie.Expires = credentials.Session.ExpiresAt
		cookie.MaxAge = int(credentials.Session.ExpiresAt.Sub(credentials.Session.CreatedAt).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

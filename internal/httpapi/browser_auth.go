package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

const maximumExchangeBodyBytes = 8 << 10

// Exchange a browser login code.
//
//	@Summary		Exchange browser login code
//	@Description	Consume a single-use browser login code and create a browser session.
//	@Tags			authentication
//	@ID				browserCodeExchange
//	@Param			request	body		api.BrowserCodeExchangeRequest	true	"Browser login code"
//	@Success		200		{object}	api.BrowserSessionResponse
//	@Failure		400		{object}	api.ErrorResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse
//	@Failure		415		{object}	api.ErrorResponse
//	@Failure		429		{object}	api.ErrorResponse
//	@Router			/api/auth/exchange [post]
func (api *v1) BrowserCodeExchange(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if api.browserStore == nil {
		writeAuthError(w, http.StatusServiceUnavailable)
		return
	}
	if len(r.Header.Values("Authorization")) > 0 || len(sessionCookieValues(r)) > 0 {
		auditAuthenticationFailure(r, "ambiguous_credentials")
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	if err := validateBrowserOrigin(r); err != nil {
		auditAuthenticationFailure(r, "cross_site_exchange")
		writeAuthError(w, http.StatusForbidden)
		return
	}
	if !api.exchangeLimiter.Allow(r.RemoteAddr) {
		w.Header().Set("Retry-After", "10")
		auditAuthenticationFailure(r, "exchange_rate_limited")
		writeAuthError(w, http.StatusTooManyRequests)
		return
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAuthError(w, http.StatusUnsupportedMediaType)
		return
	}
	var request contractapi.BrowserCodeExchangeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumExchangeBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAuthError(w, http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAuthError(w, http.StatusBadRequest)
		return
	}
	if request.Code == "" || strings.TrimSpace(request.Code) != request.Code {
		writeAuthError(w, http.StatusBadRequest)
		return
	}
	principal, err := api.authManager.ValidateBrowserCode(request.Code)
	if err != nil {
		auditAuthenticationFailure(r, "invalid_browser_code")
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	credentials, err := api.browserStore.RedeemCode(r.Context(), principal, time.Now(), api.sessionOptions(request.Remember))
	if err != nil {
		if errors.Is(err, browserauth.ErrCodeConsumed) || errors.Is(err, browserauth.ErrCodeExpired) {
			auditAuthenticationFailure(r, "invalid_browser_code")
			writeAuthError(w, http.StatusUnauthorized)
			return
		}
		log.GetLogger("auth", "audit").WithError(err).Error("Browser session exchange failed.")
		writeAuthError(w, http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, credentials)
	writeAPIJSON(w, http.StatusOK, sessionResponse(credentials.Session, credentials.CSRFToken))
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":   "browser_session_created",
		"outcome": "success",
		"scope":   principal.Scope,
	}).Info("Authentication audit event.")
}

// Log in with the local administrator password.
//
//	@Summary		Log in with administrator password
//	@Description	Verify the local administrator credential and create an opaque browser session.
//	@Tags			authentication
//	@ID				browserPasswordLogin
//	@Param			request	body		api.BrowserPasswordLoginRequest	true	"Administrator credentials"
//	@Success		200		{object}	api.BrowserSessionResponse
//	@Failure		400		{object}	api.ErrorResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse
//	@Failure		413		{object}	api.ErrorResponse
//	@Failure		415		{object}	api.ErrorResponse
//	@Failure		429		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Failure		503		{object}	api.ErrorResponse
//	@Router			/api/auth/login [post]
func (api *v1) BrowserPasswordLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	passwordStore, ok := api.browserStore.(PasswordSessionStore)
	if !ok || passwordStore == nil {
		recordBrowserAuthentication("password", "unavailable")
		writeAuthError(w, http.StatusServiceUnavailable)
		return
	}
	if len(r.Header.Values("Authorization")) > 0 || len(sessionCookieValues(r)) > 0 {
		recordBrowserAuthentication("password", "ambiguous")
		auditAuthenticationFailure(r, "ambiguous_credentials")
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	if err := validateBrowserOrigin(r); err != nil {
		recordBrowserAuthentication("password", "cross_site")
		auditAuthenticationFailure(r, "cross_site_login")
		writeAuthError(w, http.StatusForbidden)
		return
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		recordBrowserAuthentication("password", "malformed")
		writeAuthError(w, http.StatusUnsupportedMediaType)
		return
	}
	var request contractapi.BrowserPasswordLoginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumExchangeBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		var maximumBytesError *http.MaxBytesError
		if errors.As(err, &maximumBytesError) {
			recordBrowserAuthentication("password", "oversized")
			writeAuthError(w, http.StatusRequestEntityTooLarge)
			return
		}
		recordBrowserAuthentication("password", "malformed")
		writeAuthError(w, http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		recordBrowserAuthentication("password", "malformed")
		writeAuthError(w, http.StatusBadRequest)
		return
	}
	if request.Username == "" || request.Password == "" {
		recordBrowserAuthentication("password", "malformed")
		writeAuthError(w, http.StatusBadRequest)
		return
	}

	normalizedUsername, err := browserauth.NormalizeAdministratorUsername(request.Username)
	if err != nil {
		normalizedUsername = invalidPasswordUsernameKey
	}
	if reason := api.passwordLimiter.Allow(r.RemoteAddr, normalizedUsername); reason != passwordLimitAllowed {
		w.Header().Set("Retry-After", passwordRetryAfter)
		recordBrowserAuthentication("password", "rate_limited")
		auditAuthenticationFailure(r, "password_rate_limited")
		writeAuthError(w, http.StatusTooManyRequests)
		return
	}

	password := []byte(request.Password)
	request.Password = ""
	defer clearPasswordBytes(password)
	started := time.Now()
	credentials, err := passwordStore.CreatePasswordSession(r.Context(), request.Username, password, started, api.sessionOptions(request.Remember))
	passwordAuthenticationDuration.Observe(time.Since(started).Seconds())
	if err != nil {
		if errors.Is(err, browserauth.ErrInvalidCredentials) {
			recordBrowserAuthentication("password", "invalid")
			auditAuthenticationFailure(r, "invalid_credentials")
			writeAuthError(w, http.StatusUnauthorized)
			return
		}
		recordBrowserAuthentication("password", "error")
		log.GetLogger("auth", "audit").WithError(err).Error("Password browser session creation failed.")
		writeAuthError(w, http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, credentials)
	writeAPIJSON(w, http.StatusOK, sessionResponse(credentials.Session, credentials.CSRFToken))
	recordBrowserAuthentication("password", "success")
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"authentication_method": "password",
		"event":                 "browser_session_created",
		"outcome":               "success",
		"scope":                 credentials.Session.Scope,
	}).Info("Authentication audit event.")
}

func (api *v1) sessionOptions(remember bool) browserauth.SessionOptions {
	if remember {
		return browserauth.RememberedSessionOptions(api.rememberLifetime)
	}
	return browserauth.DefaultSessionOptions()
}

// Get current browser session.
//
//	@Summary		Get browser session
//	@Description	Return the active browser session and issue a CSRF token.
//	@Tags			authentication
//	@ID				browserSessionGet
//	@Success		200	{object}	api.BrowserSessionResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Router			/api/auth/session [get]
func (api *v1) BrowserSession(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	values := sessionCookieValues(r)
	if api.browserStore == nil || len(r.Header.Values("Authorization")) > 0 || len(values) != 1 {
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	session, err := api.browserStore.GetSession(r.Context(), values[0], time.Now())
	if err != nil {
		clearSessionCookie(w)
		auditAuthenticationFailure(r, "invalid_session")
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	csrfToken, err := api.browserStore.IssueCSRF(r.Context(), values[0], time.Now())
	if err != nil {
		if errors.Is(err, browserauth.ErrSessionExpired) || errors.Is(err, browserauth.ErrSessionNotFound) {
			clearSessionCookie(w)
			writeAuthError(w, http.StatusUnauthorized)
			return
		}
		writeAuthError(w, http.StatusInternalServerError)
		return
	}
	writeAPIJSON(w, http.StatusOK, sessionResponse(session, csrfToken))
}

// Logout current browser session.
//
//	@Summary		Log out browser session
//	@Description	Revoke and clear the active browser session.
//	@Tags			authentication
//	@ID				browserSessionLogout
//	@Success		204
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		403	{object}	api.ErrorResponse
//	@Router			/api/auth/logout [post]
func (api *v1) BrowserLogout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	values := sessionCookieValues(r)
	if len(r.Header.Values("Authorization")) > 0 {
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	if api.browserStore == nil || len(values) == 0 {
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(values) != 1 {
		writeAuthError(w, http.StatusUnauthorized)
		return
	}
	if _, err := api.browserStore.GetSession(r.Context(), values[0], time.Now()); err != nil {
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := validateBrowserOrigin(r); err != nil {
		auditAuthenticationFailure(r, "cross_site_request")
		writeAuthError(w, http.StatusForbidden)
		return
	}
	if err := api.browserStore.ValidateCSRF(r.Context(), values[0], r.Header.Get(csrfHeaderName), time.Now()); err != nil {
		if errors.Is(err, browserauth.ErrSessionExpired) || errors.Is(err, browserauth.ErrSessionNotFound) {
			clearSessionCookie(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		auditAuthenticationFailure(r, "invalid_csrf")
		writeAuthError(w, http.StatusForbidden)
		return
	}
	if err := api.browserStore.RevokeSession(r.Context(), values[0]); err != nil {
		writeAuthError(w, http.StatusInternalServerError)
		return
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":   "browser_session_revoked",
		"outcome": "success",
	}).Info("Authentication audit event.")
}

func sessionResponse(session browserauth.Session, csrfToken string) contractapi.BrowserSessionResponse {
	return contractapi.BrowserSessionResponse{
		Subject:   session.Subject,
		Scope:     session.Scope,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
		CSRFToken: csrfToken,
	}
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func writeAPIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAuthError(w http.ResponseWriter, status int) {
	writeAPIJSON(w, status, contractapi.ErrorResponse{
		Error: strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_")),
	})
}

func clearPasswordBytes(password []byte) {
	for i := range password {
		password[i] = 0
	}
}

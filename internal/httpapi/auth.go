package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

type Requirement struct {
	IsRequired bool
	Scope      string
}

type principalContextKey struct{}
type identityContextKey struct{}

type AuthTransport string

const (
	AuthTransportBearer  AuthTransport = "bearer"
	AuthTransportSession AuthTransport = "session"
)

type RequestIdentity struct {
	Principal auth.Principal
	Transport AuthTransport
	SessionID string
}

type BrowserSessionStore interface {
	RedeemCode(context.Context, auth.Principal, time.Time) (browserauth.Credentials, error)
	GetSession(context.Context, string, time.Time) (browserauth.Session, error)
	IssueCSRF(context.Context, string, time.Time) (string, error)
	ValidateCSRF(context.Context, string, string, time.Time) error
	RevokeSession(context.Context, string) error
}

type PasswordSessionStore interface {
	BrowserSessionStore
	CreatePasswordSession(context.Context, string, []byte, time.Time) (browserauth.Credentials, error)
}

func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	if identity, ok := IdentityFromContext(ctx); ok {
		return identity.Principal, true
	}
	principal, ok := ctx.Value(principalContextKey{}).(auth.Principal)
	return principal, ok
}

func IdentityFromContext(ctx context.Context) (RequestIdentity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(RequestIdentity)
	return identity, ok
}

func Require(
	handler http.HandlerFunc,
	requirement Requirement,
	manager *auth.Manager,
	sessions BrowserSessionStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirement.IsRequired || requirement.Scope == "" {
			handler(w, r)
			return
		}

		identity, err := authenticateRequest(r, manager, sessions, requirement.Scope)
		if err != nil {
			principal := identity.Principal
			if errors.Is(err, auth.ErrInsufficientScope) {
				log.GetLogger("auth", "audit").WithFields(logrus.Fields{
					"event":          "authorization_denied",
					"key_id":         principal.KeyID,
					"method":         r.Method,
					"outcome":        "denied",
					"required_scope": requirement.Scope,
					"route":          r.Pattern,
					"scope":          principal.Scope,
					"subject":        principal.Subject,
				}).Warn("Authentication audit event.")
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if identity.Transport == AuthTransportBearer {
				switch {
				case errors.Is(err, jwt.ErrTokenExpired):
					w.Header().Add("WWW-Authenticate", `Bearer realm="tdns", error="invalid_token", error_description="The access token expired"`)
				case errors.Is(err, jwt.ErrTokenSignatureInvalid):
					w.Header().Add("WWW-Authenticate", `Bearer realm="tdns", error="invalid_token"`)
				case errors.Is(err, jwt.ErrTokenInvalidClaims):
					w.Header().Add("WWW-Authenticate", `Bearer realm="tdns", error="invalid_token"`)
				}
			}
			auditAuthenticationFailure(r, authenticationFailureReason(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if !auth.ScopeAllows(identity.Principal.Scope, requirement.Scope) {
			auditAuthorizationDenial(r, identity.Principal, requirement.Scope)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		if identity.Transport == AuthTransportSession && isUnsafeMethod(r.Method) {
			if err := validateBrowserOrigin(r); err != nil {
				auditAuthenticationFailure(r, "cross_site_request")
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if err := sessions.ValidateCSRF(r.Context(), identity.SessionID, r.Header.Get(csrfHeaderName), time.Now()); err != nil {
				if errors.Is(err, browserauth.ErrSessionExpired) || errors.Is(err, browserauth.ErrSessionNotFound) {
					auditAuthenticationFailure(r, "expired_session")
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}
				auditAuthenticationFailure(r, "invalid_csrf")
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
		ctx = context.WithValue(ctx, principalContextKey{}, identity.Principal)
		handler(w, r.WithContext(ctx))
	}
}

var (
	errAmbiguousCredentials = errors.New("ambiguous credentials")
	errInvalidSession       = errors.New("invalid browser session")
)

func authenticateRequest(
	r *http.Request,
	manager *auth.Manager,
	sessions BrowserSessionStore,
	requiredScope string,
) (RequestIdentity, error) {
	authorization := r.Header.Values("Authorization")
	sessionValues := sessionCookieValues(r)
	if len(authorization) > 0 && len(sessionValues) > 0 {
		return RequestIdentity{}, errAmbiguousCredentials
	}
	if len(authorization) > 0 {
		identity := RequestIdentity{Transport: AuthTransportBearer}
		if len(authorization) != 1 {
			return identity, jwt.ErrTokenUnverifiable
		}
		parts := strings.Fields(authorization[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return identity, jwt.ErrTokenUnverifiable
		}
		principal, err := manager.ValidateBearer(parts[1], requiredScope)
		identity.Principal = principal
		return identity, err
	}
	if len(sessionValues) > 0 {
		identity := RequestIdentity{Transport: AuthTransportSession}
		if len(sessionValues) != 1 || sessions == nil {
			return identity, errInvalidSession
		}
		session, err := sessions.GetSession(r.Context(), sessionValues[0], time.Now())
		if err != nil {
			return identity, err
		}
		identity.SessionID = sessionValues[0]
		identity.Principal = auth.Principal{
			Subject:   session.Subject,
			Scope:     session.Scope,
			Purpose:   "browser-session",
			IssuedAt:  session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
		}
		return identity, nil
	}
	return RequestIdentity{}, errInvalidSession
}

func sessionCookieValues(r *http.Request) []string {
	values := []string{}
	for _, cookie := range r.Cookies() {
		if cookie.Name == sessionCookieName {
			values = append(values, cookie.Value)
		}
	}
	return values
}

func auditAuthorizationDenial(r *http.Request, principal auth.Principal, requiredScope string) {
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":          "authorization_denied",
		"key_id":         principal.KeyID,
		"method":         r.Method,
		"outcome":        "denied",
		"required_scope": requiredScope,
		"route":          r.Pattern,
		"scope":          principal.Scope,
		"subject":        principal.Subject,
	}).Warn("Authentication audit event.")
}

func auditAuthenticationFailure(r *http.Request, reason string) {
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":   "authentication_failed",
		"method":  r.Method,
		"outcome": "denied",
		"reason":  reason,
		"route":   r.Pattern,
	}).Warn("Authentication audit event.")
}

func authenticationFailureReason(err error) string {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return "expired"
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return "not_yet_valid"
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return "invalid_signature"
	case errors.Is(err, auth.ErrInvalidPurpose):
		return "invalid_purpose"
	case errors.Is(err, errAmbiguousCredentials):
		return "ambiguous_credentials"
	case errors.Is(err, browserauth.ErrSessionExpired):
		return "expired_session"
	case errors.Is(err, browserauth.ErrSessionNotFound), errors.Is(err, errInvalidSession):
		return "invalid_session"
	default:
		return "invalid_token"
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(value)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func AuditMutation(action string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writer := &statusWriter{ResponseWriter: w}
		handler(writer, r)
		if writer.status == 0 {
			writer.status = http.StatusOK
		}
		principal, _ := PrincipalFromContext(r.Context())
		outcome := "success"
		if writer.status >= http.StatusBadRequest {
			outcome = "failure"
		}
		log.GetLogger("auth", "audit").WithFields(logrus.Fields{
			"action":  action,
			"event":   "management_mutation",
			"key_id":  principal.KeyID,
			"method":  r.Method,
			"outcome": outcome,
			"route":   r.Pattern,
			"scope":   principal.Scope,
			"status":  writer.status,
			"subject": principal.Subject,
		}).Info("Authentication audit event.")
	}
}

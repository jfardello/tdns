package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

type Requirement struct {
	IsRequired bool
	Scope      string
}

type principalContextKey struct{}

func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(auth.Principal)
	return principal, ok
}

func Require(handler http.HandlerFunc, requirement Requirement, manager *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirement.IsRequired || requirement.Scope == "" {
			handler(w, r)
			return
		}

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			auditAuthenticationFailure(r, "missing_or_invalid_bearer")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		principal, err := manager.ValidateBearer(parts[1], requirement.Scope)
		if err != nil {
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
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="The access token expired"`)
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid token"`)
			case errors.Is(err, jwt.ErrTokenInvalidClaims):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid claims"`)
			}
			auditAuthenticationFailure(r, authenticationFailureReason(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		handler(w, r.WithContext(ctx))
	}
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

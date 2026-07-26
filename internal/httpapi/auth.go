package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

type Auth struct {
	IsRequired bool
	Scope      string
}

func Require(handler func(http.ResponseWriter, *http.Request), auth Auth) http.HandlerFunc {
	logger := log.GetLogger("api", "JWTAuth")
	return func(w http.ResponseWriter, r *http.Request) {
		// Unauthenticated access allowed
		if !auth.IsRequired || len(auth.Scope) == 0 {
			handler(w, r)
			return
		}
		//Get bearer
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		claims, err := Validate(parts[1], auth.Scope)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="The access token expired"`)
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid token"`)
			case errors.Is(err, jwt.ErrTokenInvalidClaims):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid claims"`)
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		logger.WithFields(logrus.Fields{"sub": claims["sub"], "scope": claims["scope"]}).Debug("Granting access.")
		handler(w, r)

	}
}

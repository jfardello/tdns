package httpapi

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
)

const (
	RWSCOPE string = "tdns.kubewire.net:rw"
	ROSCOPE string = "tdns.kubewire.net:rw"
)

func Validate(tokenString string, reqScope string) (jwt.MapClaims, error) {

	//tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.u1riaD1rW97opCoAuRCTy4w58Br-Zk-bh7vLiRIsrpU"

	logger := log.GetLogger("Api", "jwtValidate")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		c := config.GetRunningConfig()
		return c.Server.GetSigningKey(), nil
	})
	if err != nil {

		logger.Error(err)
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if scope, ok := claims["scope"]; ok {
			s := scope.(string)
			if s == reqScope {
				return claims, nil
			}
		}
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func IssueToken(days int, sub string) (string, error) {
	// Create a new token object specifying signing method and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"scope": RWSCOPE,
		"exp":   time.Now().Add(time.Hour * 24 * time.Duration(days)).Unix(),
		"sub":   sub,
	})
	c := config.GetRunningConfig()
	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(c.Server.GetSigningKey())

}

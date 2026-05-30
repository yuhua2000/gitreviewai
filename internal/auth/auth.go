package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

type contextKey string

const ClaimsContextKey contextKey = "jwt_claims"

type JWTClaims struct {
	jwt.RegisteredClaims
}

func GenerateToken(secret string, expiry time.Duration) (string, error) {
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gitreviewai",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(secret, tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func GinMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""

		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if tokenString == "" {
			cookie, err := c.Cookie("token")
			if err == nil {
				tokenString = cookie
			}
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.Error(types.CodeUnauthorized, "unauthorized"))
			return
		}

		claims, err := ValidateToken(secret, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.Error(types.CodeUnauthorized, "unauthorized"))
			return
		}

		c.Set(string(ClaimsContextKey), claims)
		c.Next()
	}
}

func GinLoginHandler(password, secret string, expiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request"))
			return
		}

		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(password)) != 1 {
			c.JSON(http.StatusUnauthorized, types.Error(types.CodeUnauthorized, "invalid password"))
			return
		}

		token, err := GenerateToken(secret, expiry)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to generate token"))
			return
		}

		c.SetCookie("token", token, int(expiry.Seconds()), "/", "", false, true)
		c.JSON(http.StatusOK, types.Success(types.LoginData{Token: token}))
	}
}

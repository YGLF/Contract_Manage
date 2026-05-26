package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Department  string   `json:"department"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	DataScope   string   `json:"data_scope"`
	jwt.RegisteredClaims
}

func IssueToken(secret, userID, username, department string, roles, permissions []string, dataScope string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:      userID,
		Username:    username,
		Department:  department,
		Roles:       roles,
		Permissions: permissions,
		DataScope:   dataScope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
			Issuer:    "contract-microservices",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

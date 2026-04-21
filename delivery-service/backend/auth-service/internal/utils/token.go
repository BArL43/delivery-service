package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"os"
	"time"
)

func GenerateToken(userID int, role string) (string, error) {
	secretKey := []byte(os.Getenv("JWT_SECRET"))

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

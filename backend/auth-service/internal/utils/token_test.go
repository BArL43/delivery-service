package utils

import (
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateToken(t *testing.T) {
	fakeSecret := "test-super-secret-key"
	os.Setenv("JWT_SECRET", fakeSecret)

	defer os.Unsetenv("JWT_SECRET")

	userID := 42
	role := "courier"

	tokenString, err := GenerateToken(userID, role)
	if err != nil {
		t.Fatalf("Ожидалась успешная генерация, получена ошибка: %v", err)
	}
	if tokenString == "" {
		t.Fatal("Сгенерированный токен оказался пустым")
	}

	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(fakeSecret), nil
	})

	if err != nil {
		t.Fatalf("Не удалось расшифровать токен: %v", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		t.Fatal("Токен не валиден или не содержит MapClaims")
	}

	parsedUserID := int(claims["user_id"].(float64))
	if parsedUserID != userID {
		t.Errorf("Ожидался user_id %d, получено %d", userID, parsedUserID)
	}

	if claims["role"] != role {
		t.Errorf("Ожидалась роль %s, получено %v", role, claims["role"])
	}

}

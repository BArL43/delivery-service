package handlers

import "testing"

func TestHashPassword(t *testing.T) {
	password := "my-secret-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("Ошибка при хешировании")
	}

	if hash == password {
		t.Errorf("Хещ не должне совпадать с исходным паролем")
	}

	if !CheckPasswordHash(password, hash) {
		t.Errorf("Валидный пароль не прошел проверку хеша")
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"Correct email", "user@example.com", true},
		{"Uppercase email", "USER@MAIL.RU", false},
		{"No @ sign", "mymail.com", false},
		{"No domain", "alex@", false},
		{"Empty string", "", false},
		{"Special characters", "it's_me@domain.org", true},
		{"Missing top-level domain", "admin@localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidEmail(tt.email); got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

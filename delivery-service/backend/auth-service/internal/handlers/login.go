package handlers

import (
	"projectYandexLyceumFinal/internal/models"
	"projectYandexLyceumFinal/internal/observability"
	"projectYandexLyceumFinal/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Login(c *gin.Context) {
	if db == nil {
		observability.Stats().ObserveBusiness("login", "failure")
		c.JSON(500, gin.H{"error": "База данных не инициализирована"})
		return
	}

	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		observability.Logger().Warn("auth_login_bind_failed", "error", err)
		observability.Stats().ObserveBusiness("login", "failure")
		c.JSON(400, gin.H{"error": "Неверные данные или формат"})
		return
	}

	var user models.User

	var query string
	var err error

	if strings.Contains(input.Login, "@") {
		query = `SELECT id, password_hash, role FROM users WHERE email = $1`
	} else {
		query = `SELECT id, password_hash, role FROM users WHERE phone = $1`
	}
	err = db.QueryRow(query, input.Login).Scan(&user.Id, &user.PasswordHash, &user.Role)
	if err != nil {
		observability.Logger().Warn("auth_login_invalid_credentials", "login", input.Login)
		observability.Stats().ObserveBusiness("login", "failure")
		c.JSON(401, gin.H{"error": "Неверный логин или пароль"})
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		observability.Logger().Warn("auth_login_invalid_password", "login", input.Login)
		observability.Stats().ObserveBusiness("login", "failure")
		c.JSON(401, gin.H{"error": "Неверный логин или пароль"})
		return
	}

	token, err := utils.GenerateToken(int(user.Id), user.Role)
	if err != nil {
		observability.Logger().Error("auth_login_token_failed", "error", err, "user_id", user.Id)
		observability.Stats().ObserveBusiness("login", "failure")
		c.JSON(500, gin.H{"error": "Ошибка генерации токена"})
		return
	}

	observability.Logger().Info("auth_login_success", "user_id", user.Id, "role", user.Role, "login_type", loginType(input.Login))
	observability.Stats().ObserveBusiness("login", "success")

	c.JSON(200, gin.H{
		"message": "Успешный вход",
		"token":   token,
	})
}

func loginType(login string) string {
	if strings.Contains(login, "@") {
		return "email"
	}
	return "phone"
}

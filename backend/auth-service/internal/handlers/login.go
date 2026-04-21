package handlers

import (
	"projectYandexLyceumFinal/internal/models"
	"projectYandexLyceumFinal/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Login(c *gin.Context) {
	if db == nil {
		c.JSON(500, gin.H{"error": "База данных не инициализирована"})
		return
	}

	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
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
		c.JSON(401, gin.H{"error": "Неверный логин или пароль"})
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		c.JSON(401, gin.H{"error": "Неверный логин или пароль"})
		return
	}

	token, err := utils.GenerateToken(int(user.Id), user.Role)
	if err != nil {
		c.JSON(500, gin.H{"error": "Ошибка генерации токена"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Успешный вход",
		"token":   token,
	})
}

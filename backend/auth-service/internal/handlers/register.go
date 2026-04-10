package handlers

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"projectYandexLyceumFinal/internal/models"
)

var db *sql.DB

func SetDB(database *sql.DB) {
	db = database
}

func Register(c *gin.Context) {
	if db == nil {
		c.JSON(500, gin.H{"error": "База данных не инициализирована"})
		return
	}

	var input models.RegisterInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(400, gin.H{"error": "Неверныые данные или формат"})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Внутренняя ошибка сервера(хеширование)"})
		return
	}
	newUser := models.NewUser(input.Name, input.PhoneNumber, input.Email, string(hashedPassword), "client")

	query := `INSERT INTO users (name, phone ,email, password_hash, role) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err = db.QueryRow(query, newUser.Name, newUser.PhoneNumber, newUser.Email, newUser.PasswordHash, newUser.Role).Scan(&newUser.Id)
	if err != nil {
		c.JSON(409, gin.H{"error": "Ошибка при сохранении пользователя (возможно email уже занят"})
		return
	}

	c.JSON(201, gin.H{
		"message": "Пользователь успешно зарегистрирован",
		"user_id": newUser.Id,
	})
}

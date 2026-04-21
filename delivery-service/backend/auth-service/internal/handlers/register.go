package handlers

import (
	"database/sql"
	"projectYandexLyceumFinal/internal/models"
	"projectYandexLyceumFinal/internal/observability"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

func SetDB(database *sql.DB) {
	db = database
}

func Register(c *gin.Context) {
	if db == nil {
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(500, gin.H{"error": "База данных не инициализирована"})
		return
	}

	var input models.RegisterInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		observability.Logger().Warn("auth_register_bind_failed", "error", err)
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(400, gin.H{"error": "Неверныые данные или формат"})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		observability.Logger().Error("auth_register_hash_failed", "error", err, "email", input.Email)
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(500, gin.H{"error": "Внутренняя ошибка сервера(хеширование)"})
		return
	}
	newUser := models.NewUser(input.Name, input.PhoneNumber, input.Email, string(hashedPassword), "client")

	query := `INSERT INTO users (name, phone ,email, password_hash, role) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err = db.QueryRow(query, newUser.Name, newUser.PhoneNumber, newUser.Email, newUser.PasswordHash, newUser.Role).Scan(&newUser.Id)
	if err != nil {
		observability.Logger().Warn("auth_register_save_failed", "error", err, "email", input.Email)
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(409, gin.H{"error": "Ошибка при сохранении пользователя (возможно email уже занят"})
		return
	}

	observability.Logger().Info("auth_register_success", "user_id", newUser.Id, "email", newUser.Email, "phone", newUser.PhoneNumber)
	observability.Stats().ObserveBusiness("register", "success")

	c.JSON(201, gin.H{
		"message": "Пользователь успешно зарегистрирован",
		"user_id": newUser.Id,
	})
}

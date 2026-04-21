package handlers

import (
	"database/sql"
	"fmt"
	"projectYandexLyceumFinal/internal/models"
	"projectYandexLyceumFinal/internal/observability"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
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

	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "client"
	}
	if role != "client" && role != "courier" {
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(400, gin.H{"error": "Неверная роль пользователя"})
		return
	}

	transportType := strings.TrimSpace(input.TransportType)
	if role == "courier" && transportType == "" {
		transportType = "bicycle"
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		observability.Logger().Error("auth_register_hash_failed", "error", err, "email", input.Email)
		observability.Stats().ObserveBusiness("register", "failure")
		c.JSON(500, gin.H{"error": "Внутренняя ошибка сервера(хеширование)"})
		return
	}
	newUser := models.NewUser(input.Name, input.PhoneNumber, input.Email, string(hashedPassword), role)

	query := `INSERT INTO users (name, phone ,email, password_hash, role) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err = db.QueryRow(query, newUser.Name, newUser.PhoneNumber, newUser.Email, newUser.PasswordHash, newUser.Role).Scan(&newUser.Id)
	if err != nil {
		observability.Logger().Warn("auth_register_save_failed", "error", err, "email", input.Email)
		observability.Stats().ObserveBusiness("register", "failure")
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "users_email_key":
				c.JSON(409, gin.H{"error": "Пользователь с таким email уже существует"})
				return
			case "users_phone_key":
				c.JSON(409, gin.H{"error": "Пользователь с таким телефоном уже существует"})
				return
			default:
				c.JSON(409, gin.H{"error": fmt.Sprintf("Пользователь уже существует: %s", pqErr.Constraint)})
				return
			}
		}
		c.JSON(500, gin.H{"error": "Не удалось сохранить пользователя"})
		return
	}

	courierID := ""
	if role == "courier" {
		courierID, err = registerCourierProfile(c.Request.Context(), input)
		if err != nil {
			if _, rollbackErr := db.Exec(`DELETE FROM users WHERE id = $1`, newUser.Id); rollbackErr != nil {
				observability.Logger().Error("auth_register_rollback_failed", "error", rollbackErr, "user_id", newUser.Id, "email", input.Email)
			}
			observability.Logger().Error("auth_register_courier_sync_failed", "error", err, "email", input.Email)
			observability.Stats().ObserveBusiness("register", "failure")
			c.JSON(502, gin.H{"error": fmt.Sprintf("Не удалось создать профиль курьера: %v", err)})
			return
		}
	}

	observability.Logger().Info("auth_register_success", "user_id", newUser.Id, "email", newUser.Email, "phone", newUser.PhoneNumber)
	observability.Stats().ObserveBusiness("register", "success")

	c.JSON(201, gin.H{
		"message":       "Пользователь успешно зарегистрирован",
		"user_id":       newUser.Id,
		"role":          role,
		"courier_id":    courierID,
		"transportType": transportType,
	})
}

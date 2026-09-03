package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"projectYandexLyceumFinal/internal/models"
	"projectYandexLyceumFinal/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db     *sql.DB
	tokens *utils.TokenManager
}

func New(db *sql.DB, tokens *utils.TokenManager) *Handler {
	return &Handler{db: db, tokens: tokens}
}

func (h *Handler) Register(c *gin.Context) {
	var input models.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registration data"})
		return
	}

	name := strings.TrimSpace(input.Name)
	phone := strings.TrimSpace(input.PhoneNumber)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "client"
	}
	if name == "" || phone == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, phone and email are required"})
		return
	}
	if role != "client" && role != "courier" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be client or courier"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	const query = `
		INSERT INTO users (name, phone, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var userID int64
	if err := h.db.QueryRowContext(c.Request.Context(), query, name, phone, email, string(hash), role).Scan(&userID); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "email or phone is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id": userID,
		"role":    role,
		"name":    name,
		"phone":   phone,
		"email":   email,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid login data"})
		return
	}

	login := strings.TrimSpace(input.Login)
	var user models.User
	var err error
	if strings.Contains(login, "@") {
		login = strings.ToLower(login)
		err = h.db.QueryRowContext(c.Request.Context(), `
			SELECT id, name, phone, email, password_hash, role
			FROM users
			WHERE email = $1
		`, login).Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Email, &user.PasswordHash, &user.Role)
	} else {
		err = h.db.QueryRowContext(c.Request.Context(), `
			SELECT id, name, phone, email, password_hash, role
			FROM users
			WHERE phone = $1
		`, login).Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Email, &user.PasswordHash, &user.Role)
	}
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid login or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication service unavailable"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid login or password"})
		return
	}

	token, err := h.tokens.Generate(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"role":       user.Role,
		"name":       user.Name,
		"phone":      user.PhoneNumber,
		"email":      user.Email,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userIDValue, exists := c.Get("user_id")
	userID, ok := userIDValue.(int64)
	if !exists || !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input models.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile data"})
		return
	}
	name := strings.TrimSpace(input.Name)
	phone := strings.TrimSpace(input.PhoneNumber)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if name == "" || phone == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, phone and email are required"})
		return
	}

	result, err := h.db.ExecContext(c.Request.Context(), `
		UPDATE users
		SET name = $1, phone = $2, email = $3, updated_at = NOW()
		WHERE id = $4
	`, name, phone, email, userID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "email or phone is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "profile updated",
		"name":    name,
		"phone":   phone,
		"email":   email,
	})
}

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
	if name == "" || phone == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, phone and email are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	const query = `
		INSERT INTO users (name, phone, email, password_hash, role)
		VALUES ($1, $2, $3, $4, 'client')
		RETURNING id
	`
	var userID int64
	if err := h.db.QueryRowContext(c.Request.Context(), query, name, phone, email, string(hash)).Scan(&userID); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "email or phone is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": userID})
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
		err = h.db.QueryRowContext(c.Request.Context(), `SELECT id, password_hash, role FROM users WHERE email = $1`, login).
			Scan(&user.ID, &user.PasswordHash, &user.Role)
	} else {
		err = h.db.QueryRowContext(c.Request.Context(), `SELECT id, password_hash, role FROM users WHERE phone = $1`, login).
			Scan(&user.ID, &user.PasswordHash, &user.Role)
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
	c.JSON(http.StatusOK, gin.H{"token": token, "token_type": "Bearer"})
}

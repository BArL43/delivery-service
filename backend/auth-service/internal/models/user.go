package models

type User struct {
	ID           int64
	Name         string
	PhoneNumber  string
	Email        string
	PasswordHash string
	Role         string
}

type RegisterInput struct {
	Name            string `json:"name" binding:"required,min=2,max=100"`
	PhoneNumber     string `json:"phone" binding:"required,min=10,max=20"`
	Email           string `json:"email" binding:"required,email,max=254"`
	Password        string `json:"password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,eqfield=Password"`
}

type LoginInput struct {
	Login    string `json:"login" binding:"required,max=254"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

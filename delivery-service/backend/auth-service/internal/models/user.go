package models

type User struct {
	Id           int64
	Name         string
	PhoneNumber  string
	Email        string
	PasswordHash string
	Role         string
}

type RegisterInput struct {
	Name            string `json:"name" binding:"required"`
	PhoneNumber     string `json:"phone" binding:"required,min=11,max=15"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,min=8,eqfield=Password"`
	Role            string `json:"role"`
	TransportType   string `json:"transportType"`
}

type LoginInput struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

type UpdateProfileInput struct {
	Name  string `json:"name" binding:"required"`
	Phone string `json:"phone" binding:"required,min=11,max=15"`
	Email string `json:"email" binding:"required,email"`
}

func NewUser(name, phone, email, passwordHash, role string) *User {
	return &User{
		Name:         name,
		PhoneNumber:  phone,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
	}
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	City      string `json:"city"`
	Street    string `json:"street"`
	Building  string `json:"building"`
	Apartment string `json:"apartment"`
	Comment   string `json:"comment"`
}

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	FromAddress Address   `json:"from_address"`
	ToAddress   Address   `json:"to_address"`
	Weight      float64   `json:"weight"`
	Price       float64   `json:"price"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewOrder(userID string, from, to Address, weight, price float64) Order {
	now := time.Now()
	return Order{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromAddress: from,
		ToAddress:   to,
		Weight:      weight,
		Price:       price,
		Status:      "created",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	City      string
	Street    string
	Building  string
	Apartment string
	Comment   string
}

type Order struct {
	ID          string
	UserID      string
	FromAddress Address
	ToAddress   Address
	Weight      float64
	Price       float64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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

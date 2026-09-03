package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusCreated    = "created"
	StatusAssigned   = "assigned"
	StatusAtPickup   = "at_pickup"
	StatusInProgress = "in_progress"
	StatusDelivered  = "delivered"
	StatusCancelled  = "cancelled"
)

type Address struct {
	City      string `json:"city"`
	Street    string `json:"street"`
	Building  string `json:"building,omitempty"`
	Apartment string `json:"apartment,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Order struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	FromAddress Address     `json:"from_address"`
	ToAddress   Address     `json:"to_address"`
	FromCoords  Coordinates `json:"from_coords"`
	ToCoords    Coordinates `json:"to_coords"`
	Weight      float64     `json:"weight"`
	DistanceKm  float64     `json:"distance_km"`
	Price       float64     `json:"price"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

func NewOrder(userID string, from, to Address, fromCoords, toCoords Coordinates, weight, distanceKm, price float64) Order {
	now := time.Now()
	return Order{
		ID:          uuid.NewString(),
		UserID:      userID,
		FromAddress: from,
		ToAddress:   to,
		FromCoords:  fromCoords,
		ToCoords:    toCoords,
		Weight:      weight,
		DistanceKm:  distanceKm,
		Price:       price,
		Status:      StatusCreated,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

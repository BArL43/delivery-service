package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusCreated         = "CREATED"
	StatusSearching       = "SEARCHING_COURIER"
	StatusCourierAssigned = "COURIER_ASSIGNED"
	StatusPickedUp        = "PICKED_UP"
	StatusDelivered       = "DELIVERED"
	StatusCancelled       = "CANCELLED"
)

var AllowedTransitions = map[string][]string{
	StatusCreated:         {StatusSearching, StatusCancelled},
	StatusSearching:       {StatusCourierAssigned, StatusCancelled},
	StatusCourierAssigned: {StatusPickedUp, StatusCancelled},
	StatusPickedUp:        {StatusDelivered, StatusCancelled},
	StatusDelivered:       {},
	StatusCancelled:       {},
}

func IsValidTransition(from, to string) bool {
	allowed, exists := AllowedTransitions[from]
	if !exists {
		return false
	}
	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}

type UpdateOrderStatusRequest struct {
	NewStatus string `json:"new_status" binding:"required"`
	Reason    string `json:"reason,omitempty"`
}

type UpdateOrderStatusResponse struct {
	UpdatedStatus string    `json:"updated_status"`
	ChangedAt     time.Time `json:"changed_at"`
}

type Address struct {
	City      string `json:"city" binding:"required"`
	Street    string `json:"street" binding:"required"`
	Building  string `json:"building" binding:"required"`
	Apartment string `json:"apartment"`
	Comment   string `json:"comment"`
}

type Coordinates struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

type OrderRequest struct {
	FromAddress Address     `json:"from_address" binding:"required"`
	ToAddress   Address     `json:"to_address" binding:"required"`
	FromCoords  Coordinates `json:"from_coords" binding:"required"`
	ToCoords    Coordinates `json:"to_coords" binding:"required"`
	Weight      float64     `json:"weight" binding:"required"`
	Comment     string      `json:"comment"`
	PickupTime  time.Time   `json:"pickup_time"`
}

type OrderResponse struct {
	OrderId           string        `json:"order_id"`
	InitialStatus     string        `json:"initial_status"`
	EstimatedDistance float64       `json:"estimated_distance"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	EstimatedPrice    float64       `json:"estimated_price"`
}

type OrderListResponse struct {
	Orders []OrderResponse `json:"orders"`
	Total  int             `json:"total"`
	Page   int             `json:"page"`
}

type Order struct {
	ID          string
	UserID      string
	FromAddress Address
	ToAddress   Address
	FromCoords  Coordinates
	ToCoords    Coordinates
	Weight      float64
	Price       float64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewOrder(userID string, from, to Address, fromCoords, toCoords Coordinates, weight, price float64) Order {
	now := time.Now()
	return Order{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromAddress: from,
		ToAddress:   to,
		FromCoords:  fromCoords,
		ToCoords:    toCoords,
		Weight:      weight,
		Price:       price,
		Status:      "CREATED",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

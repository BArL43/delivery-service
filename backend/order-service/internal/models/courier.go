package models

import (
	"time"

	"github.com/google/uuid"
)

// Courier represents a delivery courier.
type Courier struct {
	ID            string
	UserID        string
	Email         string
	FullName      string
	Phone         string
	TransportType string
	IsOnline      bool
	ActiveOrderID *string  // nil means no active order
	CurrentLat    *float64 // nil means no location
	CurrentLon    *float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewCourier creates a new courier with default values.
func NewCourier(userID, email, fullName, phone, transportType string) Courier {
	now := time.Now()
	return Courier{
		ID:            uuid.New().String(),
		UserID:        userID,
		Email:         email,
		FullName:      fullName,
		Phone:         phone,
		TransportType: transportType,
		IsOnline:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CourierLocation represents a GPS heartbeat from a courier.
type CourierLocation struct {
	ID         string
	CourierID  string
	Lat        float64
	Lon        float64
	Accuracy   float64 // meters
	RecordedAt time.Time
}

// NewCourierLocation creates a new location record.
func NewCourierLocation(courierID string, lat, lon, accuracy float64) CourierLocation {
	return CourierLocation{
		ID:         uuid.New().String(),
		CourierID:  courierID,
		Lat:        lat,
		Lon:        lon,
		Accuracy:   accuracy,
		RecordedAt: time.Now(),
	}
}

// Assignment represents the linking of an order to a courier.
type Assignment struct {
	ID          string
	OrderID     string
	CourierID   string
	Status      string // 'assigned' | 'accepted' | 'at_pickup' | 'in_progress' | 'delivered' | 'cancelled'
	ETAToPickup *time.Duration
	PickedUpAt  *time.Time
	DeliveredAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewAssignment creates a new assignment with 'assigned' status.
func NewAssignment(orderID, courierID string, etaToPickup time.Duration) Assignment {
	now := time.Now()
	return Assignment{
		ID:          uuid.New().String(),
		OrderID:     orderID,
		CourierID:   courierID,
		Status:      "assigned",
		ETAToPickup: &etaToPickup,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

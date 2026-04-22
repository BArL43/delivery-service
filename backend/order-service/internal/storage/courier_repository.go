package storage

import (
	"context"
	"order-service/internal/models"
)

type CourierRepository interface {
	Create(ctx context.Context, courier models.Courier) error
	GetByID(ctx context.Context, id string) (*models.Courier, error)
	GetByEmail(ctx context.Context, email string) (*models.Courier, error)
	GetByActiveOrderID(ctx context.Context, orderID string) (*models.Courier, error)
	UpdateStatus(ctx context.Context, id string, isOnline bool, transportType string) error
	UpdateLocation(ctx context.Context, id string, lat, lon float64) error
	GetActiveCourierOrder(ctx context.Context, courierID string) (*models.Order, error)
	UnassignActiveOrder(ctx context.Context, courierID string) error
	FindAvailable(ctx context.Context) ([]models.Courier, error)
	SetActiveOrder(ctx context.Context, courierID, orderID string) error
}

type AssignmentRepository interface {
	Create(ctx context.Context, a models.Assignment) error
	GetByOrderID(ctx context.Context, orderID string) (*models.Assignment, error)
	GetActiveByCourierID(ctx context.Context, courierID string) (*models.Assignment, error)
	UpdateStatus(ctx context.Context, orderID string, newStatus string) error
}

type CourierLocationRepository interface {
	Create(ctx context.Context, loc models.CourierLocation) error
}

package storage

import (
	"context"
	"order-service/internal/models"
)

type OrderRepository interface {
	Create(ctx context.Context, order models.Order) error
	GetByID(ctx context.Context, id string) (*models.Order, error)
	List(ctx context.Context) ([]models.Order, error)
	UpdateStatus(ctx context.Context, orderID string, newStatus string) error
}

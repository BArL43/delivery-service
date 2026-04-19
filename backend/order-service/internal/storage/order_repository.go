package storage

import (
	"order-service/internal/models"
	"context"
)

type OrderRepository interface {
	Create(ctx context.Context, order models.Order) error
	GetByID(ctx context.Context, id string) (*models.Order, error)
	List(ctx context.Context) ([]models.Order, error)
}

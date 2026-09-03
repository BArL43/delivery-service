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

type UserOrderRepository interface {
	OrderRepository
	ListByUser(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error)
	ListForCourier(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error)
}

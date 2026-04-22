package storage

import (
	"context"
	"order-service/internal/models"
)

type OrderRepository interface {
	Create(ctx context.Context, order models.Order) error
	GetByID(ctx context.Context, orderID string, userID string) (*models.Order, error)
	List(ctx context.Context, userID string, status string, page int, limit int, sort string) ([]models.Order, int, error)
	GetCurrentStatus(ctx context.Context, orderID string, userID string) (string, error)
	UpdateStatus(ctx context.Context, orderID string, userID string, newStatus string, reason string) error
}

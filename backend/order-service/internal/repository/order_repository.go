package repository

import (
	"api-gateway/internal/model"
	"context"
)

type OrderRepository interface {
	Create(ctx context.Context, order model.Order) error
	GetByID(ctx context.Context, id string) (*model.Order, error)
	List(ctx context.Context) ([]model.Order, error)
}

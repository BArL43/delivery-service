package storage

import (
	"context"
	"fmt"
	"time"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAssignmentRepository(pool *pgxpool.Pool) *PostgresAssignmentRepository {
	return &PostgresAssignmentRepository{
		pool: pool,
	}
}

func (r *PostgresAssignmentRepository) Create(ctx context.Context, a models.Assignment) error {
	var etaStr *string
	if a.ETAToPickup != nil {
		eta := a.ETAToPickup.String()
		etaStr = &eta
	}

	query := `
		INSERT INTO assignments (id, order_id, courier_id, status, eta_to_pickup, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query,
		a.ID,
		a.OrderID,
		a.CourierID,
		a.Status,
		etaStr,
		a.CreatedAt,
		a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert assignment: %w", err)
	}
	return nil
}

func (r *PostgresAssignmentRepository) GetByOrderID(ctx context.Context, orderID string) (*models.Assignment, error) {
	query := `
		SELECT id, order_id, courier_id, status, eta_to_pickup, picked_up_at, delivered_at, created_at, updated_at
		FROM assignments
		WHERE order_id = $1
	`
	var a models.Assignment
	var etaStr *string
	var pickedUpAt, deliveredAt *time.Time

	err := r.pool.QueryRow(ctx, query, orderID).Scan(
		&a.ID,
		&a.OrderID,
		&a.CourierID,
		&a.Status,
		&etaStr,
		&pickedUpAt,
		&deliveredAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment by order id: %w", err)
	}

	if etaStr != nil {
		eta, err := time.ParseDuration(*etaStr)
		if err == nil {
			a.ETAToPickup = &eta
		}
	}
	a.PickedUpAt = pickedUpAt
	a.DeliveredAt = deliveredAt

	return &a, nil
}

func (r *PostgresAssignmentRepository) GetActiveByCourierID(ctx context.Context, courierID string) (*models.Assignment, error) {
	query := `
		SELECT id, order_id, courier_id, status, eta_to_pickup, picked_up_at, delivered_at, created_at, updated_at
		FROM assignments
		WHERE courier_id = $1 AND status IN ('assigned', 'accepted', 'at_pickup', 'in_progress')
		ORDER BY created_at DESC
		LIMIT 1
	`
	var a models.Assignment
	var etaStr *string
	var pickedUpAt, deliveredAt *time.Time

	err := r.pool.QueryRow(ctx, query, courierID).Scan(
		&a.ID,
		&a.OrderID,
		&a.CourierID,
		&a.Status,
		&etaStr,
		&pickedUpAt,
		&deliveredAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get active assignment by courier id: %w", err)
	}

	if etaStr != nil {
		eta, err := time.ParseDuration(*etaStr)
		if err == nil {
			a.ETAToPickup = &eta
		}
	}
	a.PickedUpAt = pickedUpAt
	a.DeliveredAt = deliveredAt

	return &a, nil
}

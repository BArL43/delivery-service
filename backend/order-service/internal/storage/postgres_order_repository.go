package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{
		pool: pool,
	}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order models.Order) error {
	fromJSON, err := json.Marshal(order.FromAddress)
	if err != nil {
		return fmt.Errorf("failed to marshal from_address: %w", err)
	}

	toJSON, err := json.Marshal(order.ToAddress)
	if err != nil {
		return fmt.Errorf("failed to marshal to_address: %w", err)
	}

	query := `
		INSERT INTO orders (id, user_id, from_address, to_address, weight, price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.pool.Exec(ctx, query,
		order.ID,
		order.UserID,
		fromJSON,
		toJSON,
		order.Weight,
		order.Price,
		order.Status,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	return nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	query := `
		SELECT id, user_id, from_address, to_address, weight, price, status, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	var order models.Order
	var fromJSON, toJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.UserID,
		&fromJSON,
		&toJSON,
		&order.Weight,
		&order.Price,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get order by id: %w", err)
	}

	if err := json.Unmarshal(fromJSON, &order.FromAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal from_address: %w", err)
	}
	if err := json.Unmarshal(toJSON, &order.ToAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to_address: %w", err)
	}

	return &order, nil
}

func (r *PostgresOrderRepository) List(ctx context.Context) ([]models.Order, error) {
	query := `
		SELECT id, user_id, from_address, to_address, weight, price, status, created_at, updated_at
		FROM orders
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		var fromJSON, toJSON []byte

		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&fromJSON,
			&toJSON,
			&order.Weight,
			&order.Price,
			&order.Status,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order row: %w", err)
		}

		if err := json.Unmarshal(fromJSON, &order.FromAddress); err != nil {
			return nil, fmt.Errorf("failed to unmarshal from_address: %w", err)
		}
		if err := json.Unmarshal(toJSON, &order.ToAddress); err != nil {
			return nil, fmt.Errorf("failed to unmarshal to_address: %w", err)
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return orders, nil
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, orderID string, newStatus string) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := r.pool.Exec(ctx, query, newStatus, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found")
	}

	return nil
}

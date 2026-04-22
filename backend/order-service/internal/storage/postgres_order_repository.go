<<<<<<< HEAD
﻿package storage

import (
	"context"
	"fmt"
	"order-service/internal/models"
	"time"
=======
package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"order-service/internal/models"
>>>>>>> 6675d8db0acd470bb323dea533e3812d29de2aab

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
<<<<<<< HEAD
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	orderQuery := `
		INSERT INTO orders (id, user_id, weight, price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, orderQuery,
		order.ID, order.UserID, order.Weight, order.Price,
		order.Status, order.CreatedAt, order.UpdatedAt)
=======
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
>>>>>>> 6675d8db0acd470bb323dea533e3812d29de2aab
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

<<<<<<< HEAD
	metaQuery := `
		INSERT INTO parcel_meta (order_id, from_city, from_street, to_city, to_street, from_lat, from_lon, to_lat, to_lon)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = tx.Exec(ctx, metaQuery,
		order.ID, order.FromAddress.City, order.FromAddress.Street,
		order.ToAddress.City, order.ToAddress.Street,
		order.FromCoords.Latitude, order.FromCoords.Longitude,
		order.ToCoords.Latitude, order.ToCoords.Longitude)
	if err != nil {
		return fmt.Errorf("failed to insert parcel meta: %w", err)
	}

	historyQuery := `
		INSERT INTO order_status_history (order_id, status, actor_id, changed_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(ctx, historyQuery, order.ID, order.Status, order.UserID, order.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert status history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string, userID string) (*models.Order, error) {
	query := `
		SELECT id, user_id, weight, price, status, created_at, updated_at
		FROM orders 
		WHERE id = $1 AND user_id = $2
	`

	var o models.Order
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&o.ID, &o.UserID, &o.Weight, &o.Price, &o.Status, &o.CreatedAt, &o.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &o, nil
}

func (r *PostgresOrderRepository) List(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error) {
	countQuery := `SELECT COUNT(*) FROM orders WHERE user_id = $1`
	countArgs := []interface{}{userID}
	argID := 2

	if status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", argID)
		countArgs = append(countArgs, status)
		argID++
	}

	var total int
	err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []models.Order{}, 0, nil
	}

	query := `SELECT id, user_id, weight, price, status, created_at, updated_at FROM orders WHERE user_id = $1`
	args := []interface{}{userID}
	argID = 2

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argID)
		args = append(args, status)
		argID++
	}

	orderClause := " ORDER BY created_at DESC" // По умолчанию самые новые сверху
	if sort == "price_asc" {
		orderClause = " ORDER BY price ASC"
	} else if sort == "price_desc" {
		orderClause = " ORDER BY price DESC"
	}
	query += orderClause

	offset := (page - 1) * limit
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
=======
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
>>>>>>> 6675d8db0acd470bb323dea533e3812d29de2aab
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
<<<<<<< HEAD
		var o models.Order
		err := rows.Scan(&o.ID, &o.UserID, &o.Weight, &o.Price, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}

	return orders, total, nil
}

func (r *PostgresOrderRepository) GetCurrentStatus(ctx context.Context, orderID string, userID string) (string, error) {
	var currentStatus string
	query := `
		SELECT status 
		FROM orders 
		WHERE id = $1 AND user_id = $2
	`
	err := r.pool.QueryRow(ctx, query, orderID, userID).Scan(&currentStatus)

	if err != nil {
		return "", err
	}
	return currentStatus, nil
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, orderID string, userID string, newStatus string, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	updateOrderQuery := `
		UPDATE orders 
		SET status = $1, updated_at = $2 
		WHERE id = $3 AND user_id = $4
	`
	res, err := tx.Exec(ctx, updateOrderQuery, newStatus, now, orderID, userID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("order not found or access denied")
	}

	insertHistoryQuery := `
		INSERT INTO order_status_history (order_id, status, reason, changed_at, actor_id)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(ctx, insertHistoryQuery, orderID, newStatus, reason, now, userID)
	if err != nil {
		return fmt.Errorf("failed to insert status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
=======
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
>>>>>>> 6675d8db0acd470bb323dea533e3812d29de2aab
	}

	return nil
}

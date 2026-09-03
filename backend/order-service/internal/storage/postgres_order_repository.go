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
	return &PostgresOrderRepository{pool: pool}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order models.Order) error {
	fromAddress, err := json.Marshal(order.FromAddress)
	if err != nil {
		return fmt.Errorf("marshal from address: %w", err)
	}
	toAddress, err := json.Marshal(order.ToAddress)
	if err != nil {
		return fmt.Errorf("marshal to address: %w", err)
	}
	fromCoords, err := json.Marshal(order.FromCoords)
	if err != nil {
		return fmt.Errorf("marshal from coordinates: %w", err)
	}
	toCoords, err := json.Marshal(order.ToCoords)
	if err != nil {
		return fmt.Errorf("marshal to coordinates: %w", err)
	}

	const query = `
		INSERT INTO orders (
			id, user_id, from_address, to_address, from_coords, to_coords,
			weight, distance_km, price, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	if _, err := r.pool.Exec(ctx, query,
		order.ID, order.UserID, fromAddress, toAddress, fromCoords, toCoords,
		order.Weight, order.DistanceKm, order.Price, order.Status, order.CreatedAt, order.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	const query = `
		SELECT id, user_id, from_address, to_address, from_coords, to_coords,
		       weight, distance_km, price, status, created_at, updated_at
		FROM orders WHERE id = $1
	`
	var order models.Order
	var fromAddress, toAddress, fromCoords, toCoords []byte
	if err := r.pool.QueryRow(ctx, query, id).Scan(
		&order.ID, &order.UserID, &fromAddress, &toAddress, &fromCoords, &toCoords,
		&order.Weight, &order.DistanceKm, &order.Price, &order.Status, &order.CreatedAt, &order.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := decodeOrderJSON(&order, fromAddress, toAddress, fromCoords, toCoords); err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *PostgresOrderRepository) List(ctx context.Context) ([]models.Order, error) {
	const query = `
		SELECT id, user_id, from_address, to_address, from_coords, to_coords,
		       weight, distance_km, price, status, created_at, updated_at
		FROM orders ORDER BY created_at DESC
	`
	return r.queryOrders(ctx, query)
}

func (r *PostgresOrderRepository) ListByUser(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error) {
	countQuery := `SELECT COUNT(*) FROM orders WHERE user_id = $1`
	countArgs := []any{userID}
	if status != "" {
		countQuery += ` AND status = $2`
		countArgs = append(countArgs, status)
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	orderBy := "created_at DESC"
	switch sort {
	case "price_asc":
		orderBy = "price ASC, created_at DESC"
	case "price_desc":
		orderBy = "price DESC, created_at DESC"
	}

	args := []any{userID}
	query := `
		SELECT id, user_id, from_address, to_address, from_coords, to_coords,
		       weight, distance_km, price, status, created_at, updated_at
		FROM orders WHERE user_id = $1
	`
	if status != "" {
		query += ` AND status = $2`
		args = append(args, status)
	}
	query += " ORDER BY " + orderBy
	args = append(args, limit, (page-1)*limit)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	orders, err := r.queryOrders(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, orderID string, newStatus string) error {
	result, err := r.pool.Exec(ctx, `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`, newStatus, orderID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found")
	}
	return nil
}

func (r *PostgresOrderRepository) queryOrders(ctx context.Context, query string, args ...any) ([]models.Order, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.Order, 0)
	for rows.Next() {
		var order models.Order
		var fromAddress, toAddress, fromCoords, toCoords []byte
		if err := rows.Scan(
			&order.ID, &order.UserID, &fromAddress, &toAddress, &fromCoords, &toCoords,
			&order.Weight, &order.DistanceKm, &order.Price, &order.Status, &order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		if err := decodeOrderJSON(&order, fromAddress, toAddress, fromCoords, toCoords); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil
}

func decodeOrderJSON(order *models.Order, fromAddress, toAddress, fromCoords, toCoords []byte) error {
	if err := json.Unmarshal(fromAddress, &order.FromAddress); err != nil {
		return fmt.Errorf("decode from address: %w", err)
	}
	if err := json.Unmarshal(toAddress, &order.ToAddress); err != nil {
		return fmt.Errorf("decode to address: %w", err)
	}
	if err := json.Unmarshal(fromCoords, &order.FromCoords); err != nil {
		return fmt.Errorf("decode from coordinates: %w", err)
	}
	if err := json.Unmarshal(toCoords, &order.ToCoords); err != nil {
		return fmt.Errorf("decode to coordinates: %w", err)
	}
	return nil
}

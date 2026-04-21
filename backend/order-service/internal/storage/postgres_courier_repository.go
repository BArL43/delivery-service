package storage

import (
	"errors"
	"context"
	"encoding/json"
	"fmt"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCourierRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCourierRepository(pool *pgxpool.Pool) *PostgresCourierRepository {
	return &PostgresCourierRepository{
		pool: pool,
	}
}

func (r *PostgresCourierRepository) Create(ctx context.Context, courier models.Courier) error {
	query := `
		INSERT INTO couriers (id, user_id, email, full_name, phone, transport_type, is_online, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.pool.Exec(ctx, query,
		courier.ID,
		courier.UserID,
		courier.Email,
		courier.FullName,
		courier.Phone,
		courier.TransportType,
		courier.IsOnline,
		courier.CreatedAt,
		courier.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert courier: %w", err)
	}
	return nil
}

func (r *PostgresCourierRepository) GetByID(ctx context.Context, id string) (*models.Courier, error) {
	query := `
		SELECT id, user_id, email, full_name, phone, transport_type, is_online,
		       active_order_id, current_lat, current_lon, created_at, updated_at
		FROM couriers
		WHERE id = $1
	`
	var courier models.Courier
	var activeOrderID *string
	var currentLat, currentLon *float64

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&courier.ID,
		&courier.UserID,
		&courier.Email,
		&courier.FullName,
		&courier.Phone,
		&courier.TransportType,
		&courier.IsOnline,
		&activeOrderID,
		&currentLat,
		&currentLon,
		&courier.CreatedAt,
		&courier.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get courier by id: %w", err)
	}

	courier.ActiveOrderID = activeOrderID
	courier.CurrentLat = currentLat
	courier.CurrentLon = currentLon

	return &courier, nil
}

func (r *PostgresCourierRepository) GetByEmail(ctx context.Context, email string) (*models.Courier, error) {
	query := `
		SELECT id, user_id, email, full_name, phone, transport_type, is_online,
		       active_order_id, current_lat, current_lon, created_at, updated_at
		FROM couriers
		WHERE email = $1
	`
	var courier models.Courier
	var activeOrderID *string
	var currentLat, currentLon *float64

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&courier.ID,
		&courier.UserID,
		&courier.Email,
		&courier.FullName,
		&courier.Phone,
		&courier.TransportType,
		&courier.IsOnline,
		&activeOrderID,
		&currentLat,
		&currentLon,
		&courier.CreatedAt,
		&courier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get courier by email: %w", err)
	}

	courier.ActiveOrderID = activeOrderID
	courier.CurrentLat = currentLat
	courier.CurrentLon = currentLon

	return &courier, nil
}

func (r *PostgresCourierRepository) UpdateStatus(ctx context.Context, id string, isOnline bool, transportType string) error {
	query := `
		UPDATE couriers
		SET is_online = $2, transport_type = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, isOnline, transportType)
	if err != nil {
		return fmt.Errorf("failed to update courier status: %w", err)
	}
	return nil
}

func (r *PostgresCourierRepository) UpdateLocation(ctx context.Context, id string, lat, lon float64) error {
	updateQuery := `
		UPDATE couriers
		SET current_lat = $2, current_lon = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, updateQuery, id, lat, lon)
	if err != nil {
		return fmt.Errorf("failed to update courier location: %w", err)
	}

	insertQuery := `
		INSERT INTO courier_locations (id, courier_id, lat, lon, recorded_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	location := models.NewCourierLocation(id, lat, lon, 0)
	_, err = r.pool.Exec(ctx, insertQuery, location.ID, location.CourierID, location.Lat, location.Lon, location.RecordedAt)
	if err != nil {
		return fmt.Errorf("failed to insert courier location: %w", err)
	}

	return nil
}

func (r *PostgresCourierRepository) GetActiveCourierOrder(ctx context.Context, courierID string) (*models.Order, error) {
	query := `
		SELECT o.id, o.user_id, o.from_address, o.to_address, o.weight, o.price, o.status, o.created_at, o.updated_at
		FROM orders o
		JOIN couriers c ON o.id = c.active_order_id
		WHERE c.id = $1
	`
	var order models.Order
	var fromJSON, toJSON []byte

	err := r.pool.QueryRow(ctx, query, courierID).Scan(
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
		return nil, fmt.Errorf("failed to get active courier order: %w", err)
	}

	if err := json.Unmarshal(fromJSON, &order.FromAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal from_address: %w", err)
	}
	if err := json.Unmarshal(toJSON, &order.ToAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to_address: %w", err)
	}

	return &order, nil
}

func (r *PostgresCourierRepository) UnassignActiveOrder(ctx context.Context, courierID string) error {
	query := `
		UPDATE couriers
		SET active_order_id = NULL, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, courierID)
	if err != nil {
		return fmt.Errorf("failed to unassign active order: %w", err)
	}
	return nil
}

func (r *PostgresCourierRepository) FindAvailable(ctx context.Context) ([]models.Courier, error) {
	query := `
		SELECT id, user_id, email, full_name, phone, transport_type, is_online,
		       active_order_id, current_lat, current_lon, created_at, updated_at
		FROM couriers
		WHERE is_online = TRUE AND active_order_id IS NULL
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find available couriers: %w", err)
	}
	defer rows.Close()

	var couriers []models.Courier
	for rows.Next() {
		var courier models.Courier
		var activeOrderID *string
		var currentLat, currentLon *float64

		err := rows.Scan(
			&courier.ID,
			&courier.UserID,
			&courier.Email,
			&courier.FullName,
			&courier.Phone,
			&courier.TransportType,
			&courier.IsOnline,
			&activeOrderID,
			&currentLat,
			&currentLon,
			&courier.CreatedAt,
			&courier.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan courier row: %w", err)
		}

		courier.ActiveOrderID = activeOrderID
		courier.CurrentLat = currentLat
		courier.CurrentLon = currentLon

		couriers = append(couriers, courier)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return couriers, nil
}

func (r *PostgresCourierRepository) SetActiveOrder(ctx context.Context, courierID, orderID string) error {
	query := `
		UPDATE couriers
		SET active_order_id = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, courierID, orderID)
	if err != nil {
		return fmt.Errorf("failed to set active order: %w", err)
	}
	return nil
}


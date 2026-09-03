package storage

import (
	"context"
	"fmt"

	"order-service/internal/models"
)

func (r *PostgresCourierRepository) GetByUserID(ctx context.Context, userID string) (*models.Courier, error) {
	const query = `
		SELECT id, user_id, email, full_name, phone, transport_type, is_online,
		       active_order_id, current_lat, current_lon, created_at, updated_at
		FROM couriers
		WHERE user_id = $1
	`

	var courier models.Courier
	var activeOrderID *string
	var currentLat, currentLon *float64
	if err := r.pool.QueryRow(ctx, query, userID).Scan(
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
	); err != nil {
		return nil, fmt.Errorf("get courier by user id: %w", err)
	}

	courier.ActiveOrderID = activeOrderID
	courier.CurrentLat = currentLat
	courier.CurrentLon = currentLon
	return &courier, nil
}

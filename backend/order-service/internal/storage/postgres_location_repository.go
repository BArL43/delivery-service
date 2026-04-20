package storage

import (
	"context"
	"fmt"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresLocationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLocationRepository(pool *pgxpool.Pool) *PostgresLocationRepository {
	return &PostgresLocationRepository{
		pool: pool,
	}
}

func (r *PostgresLocationRepository) Create(ctx context.Context, loc models.CourierLocation) error {
	query := `
		INSERT INTO courier_locations (id, courier_id, lat, lon, accuracy, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		loc.ID,
		loc.CourierID,
		loc.Lat,
		loc.Lon,
		loc.Accuracy,
		loc.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert courier location: %w", err)
	}
	return nil
}

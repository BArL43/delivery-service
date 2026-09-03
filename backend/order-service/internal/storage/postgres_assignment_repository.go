package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAssignmentRepository(pool *pgxpool.Pool) *PostgresAssignmentRepository {
	return &PostgresAssignmentRepository{pool: pool}
}

func (r *PostgresAssignmentRepository) Create(ctx context.Context, a models.Assignment) error {
	var etaSeconds *float64
	if a.ETAToPickup != nil {
		seconds := a.ETAToPickup.Seconds()
		etaSeconds = &seconds
	}

	const query = `
		INSERT INTO assignments (id, order_id, courier_id, status, eta_to_pickup, created_at, updated_at)
		VALUES ($1, $2, $3, $4,
		        CASE WHEN $5::double precision IS NULL THEN NULL ELSE make_interval(secs => $5::double precision) END,
		        $6, $7)
	`
	if _, err := r.pool.Exec(ctx, query,
		a.ID, a.OrderID, a.CourierID, a.Status, etaSeconds, a.CreatedAt, a.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert assignment: %w", err)
	}
	return nil
}

func (r *PostgresAssignmentRepository) Assign(ctx context.Context, a models.Assignment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin assignment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var isOnline bool
	var activeOrderID string
	if err := tx.QueryRow(ctx, `
		SELECT is_online, COALESCE(active_order_id::text, '')
		FROM couriers
		WHERE id = $1
		FOR UPDATE
	`, a.CourierID).Scan(&isOnline, &activeOrderID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCourierNotFound
		}
		return fmt.Errorf("lock courier: %w", err)
	}
	if !isOnline {
		return ErrCourierUnavailable
	}
	if activeOrderID != "" {
		return ErrCourierBusy
	}

	var orderStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, a.OrderID).Scan(&orderStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		return fmt.Errorf("lock order: %w", err)
	}
	if orderStatus != models.StatusCreated {
		return ErrOrderNotAssignable
	}

	var etaSeconds *float64
	if a.ETAToPickup != nil {
		seconds := a.ETAToPickup.Seconds()
		etaSeconds = &seconds
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO assignments (id, order_id, courier_id, status, eta_to_pickup, created_at, updated_at)
		VALUES ($1, $2, $3, $4,
		        CASE WHEN $5::double precision IS NULL THEN NULL ELSE make_interval(secs => $5::double precision) END,
		        $6, $7)
	`, a.ID, a.OrderID, a.CourierID, a.Status, etaSeconds, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrOrderAlreadyAssigned
		}
		return fmt.Errorf("insert assignment: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE couriers
		SET active_order_id = $2, updated_at = NOW()
		WHERE id = $1
	`, a.CourierID, a.OrderID); err != nil {
		return fmt.Errorf("set courier active order: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`, a.OrderID, models.StatusAssigned); err != nil {
		return fmt.Errorf("mark order assigned: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit assignment transaction: %w", err)
	}
	return nil
}

func (r *PostgresAssignmentRepository) GetByOrderID(ctx context.Context, orderID string) (*models.Assignment, error) {
	const query = `
		SELECT id, order_id, courier_id, status, EXTRACT(EPOCH FROM eta_to_pickup),
		       picked_up_at, delivered_at, created_at, updated_at
		FROM assignments
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanAssignment(r.pool.QueryRow(ctx, query, orderID), "get assignment by order id")
}

func (r *PostgresAssignmentRepository) GetActiveByCourierID(ctx context.Context, courierID string) (*models.Assignment, error) {
	const query = `
		SELECT id, order_id, courier_id, status, EXTRACT(EPOCH FROM eta_to_pickup),
		       picked_up_at, delivered_at, created_at, updated_at
		FROM assignments
		WHERE courier_id = $1 AND status IN ('assigned', 'at_pickup', 'in_progress')
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanAssignment(r.pool.QueryRow(ctx, query, courierID), "get active assignment by courier id")
}

func (r *PostgresAssignmentRepository) UpdateStatus(ctx context.Context, orderID string, newStatus string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE assignments
		SET status = $1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM assignments WHERE order_id = $2 ORDER BY created_at DESC LIMIT 1
		)
	`, newStatus, orderID)
	if err != nil {
		return fmt.Errorf("update assignment status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

func (r *PostgresAssignmentRepository) Transition(ctx context.Context, orderID, courierID, newStatus string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin status transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var assignmentID, assignedCourierID, currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT id::text, courier_id::text, status
		FROM assignments
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, orderID).Scan(&assignmentID, &assignedCourierID, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAssignmentNotFound
		}
		return fmt.Errorf("lock assignment: %w", err)
	}
	if assignedCourierID != courierID {
		return ErrAssignmentOwnership
	}
	if currentStatus == newStatus {
		return tx.Commit(ctx)
	}
	if !validAssignmentTransition(currentStatus, newStatus) {
		return ErrInvalidTransition
	}

	if _, err := tx.Exec(ctx, `
		UPDATE assignments
		SET status = $1,
		    picked_up_at = CASE
		        WHEN $1 = 'in_progress' AND picked_up_at IS NULL THEN NOW()
		        ELSE picked_up_at
		    END,
		    delivered_at = CASE
		        WHEN $1 = 'delivered' AND delivered_at IS NULL THEN NOW()
		        ELSE delivered_at
		    END,
		    updated_at = NOW()
		WHERE id = $2::uuid
	`, newStatus, assignmentID); err != nil {
		return fmt.Errorf("update assignment transition: %w", err)
	}
	result, err := tx.Exec(ctx, `UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, orderID, newStatus)
	if err != nil {
		return fmt.Errorf("update order transition: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrderNotFound
	}

	if newStatus == models.StatusDelivered || newStatus == models.StatusCancelled {
		if _, err := tx.Exec(ctx, `
			UPDATE couriers
			SET active_order_id = NULL, updated_at = NOW()
			WHERE id = $1 AND active_order_id = $2
		`, courierID, orderID); err != nil {
			return fmt.Errorf("release courier active order: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit status transition: %w", err)
	}
	return nil
}

func (r *PostgresAssignmentRepository) scanAssignment(row pgx.Row, operation string) (*models.Assignment, error) {
	var a models.Assignment
	var etaSeconds sql.NullFloat64
	var pickedUpAt, deliveredAt *time.Time
	if err := row.Scan(
		&a.ID,
		&a.OrderID,
		&a.CourierID,
		&a.Status,
		&etaSeconds,
		&pickedUpAt,
		&deliveredAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	if etaSeconds.Valid {
		duration := time.Duration(etaSeconds.Float64 * float64(time.Second))
		a.ETAToPickup = &duration
	}
	a.PickedUpAt = pickedUpAt
	a.DeliveredAt = deliveredAt
	return &a, nil
}

func validAssignmentTransition(currentStatus, newStatus string) bool {
	switch currentStatus {
	case models.StatusAssigned:
		return newStatus == models.StatusAtPickup || newStatus == models.StatusCancelled
	case models.StatusAtPickup:
		return newStatus == models.StatusInProgress || newStatus == models.StatusCancelled
	case models.StatusInProgress:
		return newStatus == models.StatusDelivered || newStatus == models.StatusCancelled
	default:
		return false
	}
}

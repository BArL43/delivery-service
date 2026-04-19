package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"order-service/internal/models"
	"order-service/internal/storage"

	"github.com/jackc/pgx/v5/pgconn"
)

type CourierHandler struct {
	courierRepo    storage.CourierRepository
	assignmentRepo storage.AssignmentRepository
}

func NewCourierHandler(
	courierRepo storage.CourierRepository,
	assignmentRepo storage.AssignmentRepository,
) *CourierHandler {
	return &CourierHandler{
		courierRepo:    courierRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ToggleAvailability handles POST /couriers/availability
func (h *CourierHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CourierID     string `json:"courier_id"`
		IsOnline      bool   `json:"is_online"`
		TransportType string `json:"transport_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CourierID == "" {
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	if err := h.courierRepo.UpdateStatus(r.Context(), req.CourierID, req.IsOnline, req.TransportType); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update availability")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"courier_id":     req.CourierID,
		"is_online":      req.IsOnline,
		"transport_type": req.TransportType,
	})
}

// UpdateLocation handles POST /couriers/location
func (h *CourierHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CourierID string  `json:"courier_id"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CourierID == "" {
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	if err := h.courierRepo.UpdateLocation(r.Context(), req.CourierID, req.Lat, req.Lon); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update location")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AssignOrder handles POST /orders/{orderId}/assign
func (h *CourierHandler) AssignOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	path = strings.TrimSuffix(path, "/")
	orderId := strings.TrimSuffix(path, "/assign")
	orderId = strings.TrimSuffix(orderId, "/")

	if orderId == "" {
		jsonError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	var req struct {
		CourierID string `json:"courier_id"`
		Mode      string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var courierID string

	if req.Mode == "auto" {
		couriers, err := h.courierRepo.FindAvailable(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to find available couriers")
			return
		}
		if len(couriers) == 0 {
			jsonError(w, http.StatusNotFound, "no available couriers")
			return
		}

		// Pick the first available courier (closest by proximity)
		courierID = couriers[0].ID
	} else {
		if req.CourierID == "" {
			jsonError(w, http.StatusBadRequest, "courier_id is required for manual mode")
			return
		}
		courierID = req.CourierID
	}

	// Create assignment
	assignment := models.NewAssignment(orderId, courierID, 0)
	if err := h.assignmentRepo.Create(r.Context(), assignment); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			jsonError(w, http.StatusConflict, "order is already assigned to a courier")
			return
		}
		jsonError(w, http.StatusInternalServerError, "failed to create assignment")
		return
	}

	// Update courier's active order
	if err := h.courierRepo.SetActiveOrder(r.Context(), courierID, orderId); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to set active order")
		return
	}

	// Get courier for ETA calculation
	courier, err := h.courierRepo.GetByID(r.Context(), courierID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get courier")
		return
	}

	// Compute ETA using default distance of 5 km
	eta := computeETA(courier.TransportType, 5.0)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"courier_id": courierID,
		"eta":        eta,
	})
}

// GetActiveOrder handles GET /couriers/{courierId}/active-order
func (h *CourierHandler) GetActiveOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/couriers/")
	path = strings.TrimSuffix(path, "/")
	courierId := strings.TrimSuffix(path, "/active-order")
	courierId = strings.TrimSuffix(courierId, "/")

	if courierId == "" {
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	order, err := h.courierRepo.GetActiveCourierOrder(r.Context(), courierId)
	if err != nil {
		jsonError(w, http.StatusNotFound, "no active order for this courier")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"order_id":     order.ID,
		"from_address": order.FromAddress,
		"to_address":   order.ToAddress,
	})
}

func computeETA(transportType string, distanceKm float64) string {
	var speedKmh float64
	switch transportType {
	case "bicycle":
		speedKmh = 15
	case "scooter":
		speedKmh = 25
	case "car":
		speedKmh = 40
	default:
		speedKmh = 15
	}

	minutes := int(math.Round(distanceKm / speedKmh * 60))
	return fmt.Sprintf("%dm", minutes)
}

func jsonResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

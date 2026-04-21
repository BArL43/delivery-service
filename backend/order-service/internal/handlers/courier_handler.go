package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"order-service/internal/models"
	"order-service/internal/observability"
	"order-service/internal/storage"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/google/uuid"
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

// RegisterCourier handles POST /api/v1/couriers/register
func (h *CourierHandler) RegisterCourier(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email         string `json:"email"`
		FullName      string `json:"full_name"`
		Phone         string `json:"phone"`
		TransportType string `json:"transport_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		observability.Logger().Warn("courier_register_decode_error", "error", err)
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.TransportType = strings.TrimSpace(req.TransportType)
	if req.TransportType == "" {
		req.TransportType = "bicycle"
	}

	if req.Email == "" || req.FullName == "" || req.Phone == "" {
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusBadRequest, "email, full_name and phone are required")
		return
	}

	courier := models.NewCourier(uuid.NewString(), req.Email, req.FullName, req.Phone, req.TransportType)
	if err := h.courierRepo.Create(r.Context(), courier); err != nil {
		observability.Logger().Error("courier_register_create_failed", "error", err, "email", req.Email)
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to create courier profile")
		return
	}

	observability.Logger().Info("courier_registered", "courier_id", courier.ID, "email", courier.Email, "transport_type", courier.TransportType)
	observability.Stats().ObserveBusiness("courier_register", "success")

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"courier_id":     courier.ID,
		"user_id":        courier.UserID,
		"email":          courier.Email,
		"full_name":      courier.FullName,
		"phone":          courier.Phone,
		"transport_type": courier.TransportType,
		"is_online":      courier.IsOnline,
	})
}

// GetCourierByEmail handles GET /api/v1/couriers/by-email?email=...
func (h *CourierHandler) GetCourierByEmail(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		observability.Stats().ObserveBusiness("courier_lookup", "failure")
		jsonError(w, http.StatusBadRequest, "email is required")
		return
	}

	courier, err := h.courierRepo.GetByEmail(r.Context(), email)
	if err != nil {
		observability.Logger().Warn("courier_lookup_failed", "error", err, "email", email)
		observability.Stats().ObserveBusiness("courier_lookup", "failure")
		jsonError(w, http.StatusNotFound, "courier not found")
		return
	}

	observability.Stats().ObserveBusiness("courier_lookup", "success")
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"courier_id":     courier.ID,
		"user_id":        courier.UserID,
		"email":          courier.Email,
		"full_name":      courier.FullName,
		"phone":          courier.Phone,
		"transport_type": courier.TransportType,
		"is_online":      courier.IsOnline,
		"active_order_id": courier.ActiveOrderID,
	})
}

// ToggleAvailability handles POST /couriers/availability
func (h *CourierHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CourierID     string `json:"courier_id"`
		IsOnline      bool   `json:"is_online"`
		TransportType string `json:"transport_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		observability.Logger().Warn("courier_availability_decode_error", "error", err)
		observability.Stats().ObserveBusiness("courier_availability_update", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CourierID == "" {
		observability.Stats().ObserveBusiness("courier_availability_update", "failure")
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	if err := h.courierRepo.UpdateStatus(r.Context(), req.CourierID, req.IsOnline, req.TransportType); err != nil {
		observability.Logger().Error("courier_availability_update_failed", "error", err, "courier_id", req.CourierID)
		observability.Stats().ObserveBusiness("courier_availability_update", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to update availability")
		return
	}

	observability.Logger().Info("courier_availability_updated", "courier_id", req.CourierID, "is_online", req.IsOnline, "transport_type", req.TransportType)
	observability.Stats().ObserveBusiness("courier_availability_update", "success")

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
		observability.Logger().Warn("courier_location_decode_error", "error", err)
		observability.Stats().ObserveBusiness("courier_location_update", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CourierID == "" {
		observability.Stats().ObserveBusiness("courier_location_update", "failure")
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	if err := h.courierRepo.UpdateLocation(r.Context(), req.CourierID, req.Lat, req.Lon); err != nil {
		observability.Logger().Error("courier_location_update_failed", "error", err, "courier_id", req.CourierID)
		observability.Stats().ObserveBusiness("courier_location_update", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to update location")
		return
	}

	observability.Logger().Info("courier_location_updated", "courier_id", req.CourierID, "lat", req.Lat, "lon", req.Lon)
	observability.Stats().ObserveBusiness("courier_location_update", "success")

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AssignOrder handles POST /orders/{orderId}/assign
func (h *CourierHandler) AssignOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	path = strings.TrimSuffix(path, "/")
	orderId := strings.TrimSuffix(path, "/assign")
	orderId = strings.TrimSuffix(orderId, "/")

	if orderId == "" {
		observability.Stats().ObserveBusiness("courier_assign", "failure")
		jsonError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	var req struct {
		CourierID string `json:"courier_id"`
		Mode      string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		observability.Logger().Warn("courier_assign_decode_error", "error", err)
		observability.Stats().ObserveBusiness("courier_assign", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var courierID string

	if req.Mode == "auto" {
		couriers, err := h.courierRepo.FindAvailable(r.Context())
		if err != nil {
			observability.Logger().Error("courier_assign_find_available_failed", "error", err, "order_id", orderId)
			observability.Stats().ObserveBusiness("courier_assign", "failure")
			jsonError(w, http.StatusInternalServerError, "failed to find available couriers")
			return
		}
		if len(couriers) == 0 {
			observability.Stats().ObserveBusiness("courier_assign", "failure")
			jsonError(w, http.StatusNotFound, "no available couriers")
			return
		}

		// Pick the first available courier (closest by proximity)
		courierID = couriers[0].ID
	} else {
		if req.CourierID == "" {
			observability.Stats().ObserveBusiness("courier_assign", "failure")
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
			observability.Stats().ObserveBusiness("courier_assign", "failure")
			jsonError(w, http.StatusConflict, "order is already assigned to a courier")
			return
		}
		observability.Logger().Error("courier_assign_create_failed", "error", err, "order_id", orderId, "courier_id", courierID)
		observability.Stats().ObserveBusiness("courier_assign", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to create assignment")
		return
	}

	// Update courier's active order
	if err := h.courierRepo.SetActiveOrder(r.Context(), courierID, orderId); err != nil {
		observability.Logger().Error("courier_assign_set_active_failed", "error", err, "order_id", orderId, "courier_id", courierID)
		observability.Stats().ObserveBusiness("courier_assign", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to set active order")
		return
	}

	// Get courier for ETA calculation
	courier, err := h.courierRepo.GetByID(r.Context(), courierID)
	if err != nil {
		observability.Logger().Error("courier_assign_get_courier_failed", "error", err, "order_id", orderId, "courier_id", courierID)
		observability.Stats().ObserveBusiness("courier_assign", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to get courier")
		return
	}

	// Compute ETA using default distance of 5 km
	eta := computeETA(courier.TransportType, 5.0)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"courier_id": courierID,
		"eta":        eta,
	})

	observability.Logger().Info("courier_assigned", "order_id", orderId, "courier_id", courierID, "eta", eta, "mode", req.Mode)
	observability.Stats().ObserveBusiness("courier_assign", "success")
}

// GetActiveOrder handles GET /couriers/{courierId}/active-order
func (h *CourierHandler) GetActiveOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/couriers/")
	path = strings.TrimSuffix(path, "/")
	courierId := strings.TrimSuffix(path, "/active-order")
	courierId = strings.TrimSuffix(courierId, "/")

	if courierId == "" {
		observability.Stats().ObserveBusiness("courier_active_order_lookup", "failure")
		jsonError(w, http.StatusBadRequest, "courier_id is required")
		return
	}

	order, err := h.courierRepo.GetActiveCourierOrder(r.Context(), courierId)
	if err != nil {
		observability.Stats().ObserveBusiness("courier_active_order_lookup", "failure")
		jsonError(w, http.StatusNotFound, "no active order for this courier")
		return
	}

	observability.Stats().ObserveBusiness("courier_active_order_lookup", "success")

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

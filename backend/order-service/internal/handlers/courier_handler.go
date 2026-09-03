package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"order-service/internal/middleware"
	"order-service/internal/models"
	"order-service/internal/observability"
	"order-service/internal/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CourierHandler struct {
	courierRepo    storage.CourierRepository
	assignmentRepo storage.AssignmentRepository
	orderRepo      storage.OrderRepository
}

func NewCourierHandler(
	courierRepo storage.CourierRepository,
	assignmentRepo storage.AssignmentRepository,
	orderRepo storage.OrderRepository,
) *CourierHandler {
	return &CourierHandler{courierRepo: courierRepo, assignmentRepo: assignmentRepo, orderRepo: orderRepo}
}

func (h *CourierHandler) RegisterCourier(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	role, _ := middleware.Role(r.Context())
	if role != "courier" {
		jsonError(w, http.StatusForbidden, "courier role is required")
		return
	}

	var req struct {
		Email         string `json:"email"`
		FullName      string `json:"full_name"`
		Phone         string `json:"phone"`
		TransportType string `json:"transport_type"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.TransportType = strings.TrimSpace(req.TransportType)
	if req.TransportType == "" {
		req.TransportType = "bicycle"
	}
	if req.Email == "" || req.FullName == "" || req.Phone == "" || !validTransport(req.TransportType) {
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusBadRequest, "valid email, full_name, phone and transport_type are required")
		return
	}

	if existing, err := h.courierRepo.GetByUserID(r.Context(), userID); err == nil {
		observability.Stats().ObserveBusiness("courier_register", "success")
		writeCourier(w, http.StatusOK, existing)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		observability.Logger().Error("courier_register_user_lookup_failed", "error", err, "user_id", userID)
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to check courier profile")
		return
	}

	if existing, err := h.courierRepo.GetByEmail(r.Context(), req.Email); err == nil {
		if existing.UserID != userID {
			observability.Stats().ObserveBusiness("courier_register", "failure")
			jsonError(w, http.StatusConflict, "email is already used by another courier")
			return
		}
		writeCourier(w, http.StatusOK, existing)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		observability.Logger().Error("courier_register_email_lookup_failed", "error", err, "email", req.Email)
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to check courier email")
		return
	}

	courier := models.NewCourier(userID, req.Email, req.FullName, req.Phone, req.TransportType)
	if err := h.courierRepo.Create(r.Context(), courier); err != nil {
		observability.Logger().Error("courier_register_create_failed", "error", err, "user_id", userID)
		observability.Stats().ObserveBusiness("courier_register", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to create courier profile")
		return
	}

	observability.Logger().Info("courier_registered", "courier_id", courier.ID, "user_id", userID)
	observability.Stats().ObserveBusiness("courier_register", "success")
	writeCourier(w, http.StatusCreated, &courier)
}

func (h *CourierHandler) GetCourierByEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	role, _ := middleware.Role(r.Context())
	if role != "courier" {
		jsonError(w, http.StatusForbidden, "courier role is required")
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	if email == "" {
		jsonError(w, http.StatusBadRequest, "email is required")
		return
	}

	courier, err := h.courierRepo.GetByEmail(r.Context(), email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && courier.UserID != userID) {
		jsonError(w, http.StatusNotFound, "courier not found")
		return
	}
	if err != nil {
		observability.Logger().Error("courier_lookup_failed", "error", err, "user_id", userID)
		jsonError(w, http.StatusInternalServerError, "failed to get courier")
		return
	}
	writeCourier(w, http.StatusOK, courier)
}

func (h *CourierHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	courier, ok := h.loadOwnedCourier(w, r)
	if !ok {
		return
	}
	var req struct {
		CourierID     string `json:"courier_id"`
		IsOnline      bool   `json:"is_online"`
		TransportType string `json:"transport_type"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestedID := strings.TrimSpace(req.CourierID); requestedID != "" && requestedID != courier.ID {
		jsonError(w, http.StatusForbidden, "cannot update another courier")
		return
	}
	req.TransportType = strings.TrimSpace(req.TransportType)
	if req.TransportType == "" {
		req.TransportType = courier.TransportType
	}
	if !validTransport(req.TransportType) {
		jsonError(w, http.StatusBadRequest, "unsupported transport_type")
		return
	}
	if err := h.courierRepo.UpdateStatus(r.Context(), courier.ID, req.IsOnline, req.TransportType); err != nil {
		observability.Logger().Error("courier_availability_update_failed", "error", err, "courier_id", courier.ID)
		jsonError(w, http.StatusInternalServerError, "failed to update availability")
		return
	}
	observability.Stats().ObserveBusiness("courier_availability_update", "success")
	jsonResponse(w, http.StatusOK, map[string]any{
		"courier_id": courier.ID, "is_online": req.IsOnline, "transport_type": req.TransportType,
	})
}

func (h *CourierHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	courier, ok := h.loadOwnedCourier(w, r)
	if !ok {
		return
	}
	var req struct {
		CourierID string  `json:"courier_id"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestedID := strings.TrimSpace(req.CourierID); requestedID != "" && requestedID != courier.ID {
		jsonError(w, http.StatusForbidden, "cannot update another courier")
		return
	}
	if !validLatLon(req.Lat, req.Lon) {
		jsonError(w, http.StatusBadRequest, "invalid latitude or longitude")
		return
	}
	if err := h.courierRepo.UpdateLocation(r.Context(), courier.ID, req.Lat, req.Lon); err != nil {
		observability.Logger().Error("courier_location_update_failed", "error", err, "courier_id", courier.ID)
		jsonError(w, http.StatusInternalServerError, "failed to update location")
		return
	}
	observability.Stats().ObserveBusiness("courier_location_update", "success")
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *CourierHandler) AssignOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	role, ok := middleware.Role(r.Context())
	if !ok || (role != "client" && role != "courier") {
		jsonError(w, http.StatusForbidden, "unsupported role")
		return
	}

	orderID := strings.TrimSpace(r.PathValue("orderId"))
	if _, err := uuid.Parse(orderID); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid order_id")
		return
	}
	order, err := h.orderRepo.GetByID(r.Context(), orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get order")
		return
	}

	var req struct {
		CourierID string `json:"courier_id"`
		Mode      string `json:"mode"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "auto"
	}

	var courierID string
	switch mode {
	case "auto":
		if role != "client" {
			jsonError(w, http.StatusForbidden, "auto assignment is available to clients only")
			return
		}
		if order.UserID != userID {
			jsonError(w, http.StatusNotFound, "order not found")
			return
		}
		couriers, err := h.courierRepo.FindAvailable(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to find available couriers")
			return
		}
		if len(couriers) == 0 {
			jsonError(w, http.StatusNotFound, "no available couriers")
			return
		}
		courierID = couriers[0].ID
	case "manual":
		if role != "courier" {
			jsonError(w, http.StatusForbidden, "manual assignment is available to couriers only")
			return
		}
		ownedCourier, err := h.courierRepo.GetByUserID(r.Context(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "courier profile not found")
			return
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to get courier profile")
			return
		}
		requestedID := strings.TrimSpace(req.CourierID)
		if requestedID != "" && requestedID != ownedCourier.ID {
			jsonError(w, http.StatusForbidden, "cannot assign an order to another courier")
			return
		}
		courierID = ownedCourier.ID
	default:
		jsonError(w, http.StatusBadRequest, "mode must be auto or manual")
		return
	}

	courier, err := h.courierRepo.GetByID(r.Context(), courierID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "courier not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get courier")
		return
	}
	etaDuration := computeETADuration(courier.TransportType, order.DistanceKm)
	assignment := models.NewAssignment(orderID, courierID, etaDuration)
	if err := h.assignmentRepo.Assign(r.Context(), assignment); err != nil {
		switch {
		case errors.Is(err, storage.ErrCourierNotFound):
			jsonError(w, http.StatusNotFound, "courier not found")
		case errors.Is(err, storage.ErrOrderNotFound):
			jsonError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, storage.ErrCourierBusy), errors.Is(err, storage.ErrCourierUnavailable),
			errors.Is(err, storage.ErrOrderAlreadyAssigned), errors.Is(err, storage.ErrOrderNotAssignable):
			jsonError(w, http.StatusConflict, err.Error())
		default:
			observability.Logger().Error("courier_assign_failed", "error", err, "order_id", orderID, "courier_id", courierID)
			jsonError(w, http.StatusInternalServerError, "failed to assign order")
		}
		return
	}

	observability.Logger().Info("courier_assigned", "order_id", orderID, "courier_id", courierID, "mode", mode)
	observability.Stats().ObserveBusiness("courier_assign", "success")
	jsonResponse(w, http.StatusOK, map[string]any{"courier_id": courierID, "eta": computeETA(courier.TransportType, order.DistanceKm)})
}

func (h *CourierHandler) GetActiveOrder(w http.ResponseWriter, r *http.Request) {
	courier, ok := h.loadOwnedCourier(w, r)
	if !ok {
		return
	}
	requestedID := strings.TrimSpace(r.PathValue("courierId"))
	if requestedID != "" && requestedID != courier.ID {
		jsonError(w, http.StatusForbidden, "cannot read another courier")
		return
	}
	order, err := h.courierRepo.GetActiveCourierOrder(r.Context(), courier.ID)
	if err != nil || order == nil {
		jsonError(w, http.StatusNotFound, "no active order for this courier")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"order_id": order.ID, "from_address": order.FromAddress, "to_address": order.ToAddress,
	})
}

func (h *CourierHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	courier, ok := h.loadOwnedCourier(w, r)
	if !ok {
		return
	}
	orderID := strings.TrimSpace(r.PathValue("orderId"))
	if _, err := uuid.Parse(orderID); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid order_id")
		return
	}
	var req struct {
		CourierID string `json:"courier_id"`
		Status    string `json:"status"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestedID := strings.TrimSpace(req.CourierID); requestedID != "" && requestedID != courier.ID {
		jsonError(w, http.StatusForbidden, "cannot update another courier's order")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != models.StatusAtPickup && status != models.StatusInProgress && status != models.StatusDelivered && status != models.StatusCancelled {
		jsonError(w, http.StatusBadRequest, "unsupported status")
		return
	}

	if err := h.assignmentRepo.Transition(r.Context(), orderID, courier.ID, status); err != nil {
		switch {
		case errors.Is(err, storage.ErrAssignmentNotFound), errors.Is(err, storage.ErrOrderNotFound):
			jsonError(w, http.StatusNotFound, "assignment not found")
		case errors.Is(err, storage.ErrAssignmentOwnership):
			jsonError(w, http.StatusForbidden, "order is assigned to another courier")
		case errors.Is(err, storage.ErrInvalidTransition):
			jsonError(w, http.StatusConflict, "invalid status transition")
		default:
			observability.Logger().Error("courier_order_status_update_failed", "error", err, "order_id", orderID, "courier_id", courier.ID)
			jsonError(w, http.StatusInternalServerError, "failed to update order status")
		}
		return
	}

	observability.Logger().Info("courier_order_status_updated", "order_id", orderID, "courier_id", courier.ID, "status", status)
	observability.Stats().ObserveBusiness("courier_order_status_update", "success")
	jsonResponse(w, http.StatusOK, map[string]any{
		"order_id": orderID, "courier_id": courier.ID, "status": status, "reason": strings.TrimSpace(req.Reason),
	})
}

func (h *CourierHandler) loadOwnedCourier(w http.ResponseWriter, r *http.Request) (*models.Courier, bool) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	role, _ := middleware.Role(r.Context())
	if role != "courier" {
		jsonError(w, http.StatusForbidden, "courier role is required")
		return nil, false
	}
	courier, err := h.courierRepo.GetByUserID(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "courier profile not found")
		return nil, false
	}
	if err != nil {
		observability.Logger().Error("courier_owner_lookup_failed", "error", err, "user_id", userID)
		jsonError(w, http.StatusInternalServerError, "failed to get courier profile")
		return nil, false
	}
	return courier, true
}

func writeCourier(w http.ResponseWriter, status int, courier *models.Courier) {
	jsonResponse(w, status, map[string]any{
		"courier_id":      courier.ID,
		"user_id":         courier.UserID,
		"email":           courier.Email,
		"full_name":       courier.FullName,
		"phone":           courier.Phone,
		"transport_type":  courier.TransportType,
		"is_online":       courier.IsOnline,
		"active_order_id": courier.ActiveOrderID,
	})
}

func validTransport(transportType string) bool {
	switch transportType {
	case "bicycle", "scooter", "car":
		return true
	default:
		return false
	}
}

func validLatLon(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) &&
		lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

func computeETADuration(transportType string, distanceKm float64) time.Duration {
	var speedKmh float64
	switch transportType {
	case "scooter":
		speedKmh = 25
	case "car":
		speedKmh = 40
	default:
		speedKmh = 15
	}
	minutes := int(math.Round(distanceKm / speedKmh * 60))
	if minutes < 1 {
		minutes = 1
	}
	return time.Duration(minutes) * time.Minute
}

func computeETA(transportType string, distanceKm float64) string {
	return fmt.Sprintf("%dm", int(computeETADuration(transportType, distanceKm)/time.Minute))
}

func jsonResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

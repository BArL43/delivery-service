package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"order-service/internal/distance"
	"order-service/internal/middleware"
	"order-service/internal/models"
	"order-service/internal/observability"
	"order-service/internal/pricing"
	"order-service/internal/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxJSONBody = 1 << 20

type CreateOrderRequest struct {
	FromAddress models.Address     `json:"from_address"`
	ToAddress   models.Address     `json:"to_address"`
	FromCoords  models.Coordinates `json:"from_coords"`
	ToCoords    models.Coordinates `json:"to_coords"`
	Weight      float64            `json:"weight"`
}

type OrdersHandler struct {
	repo storage.UserOrderRepository
	calc *pricing.Calculator
}

func NewOrdersHandler(repo storage.UserOrderRepository, calc *pricing.Calculator) *OrdersHandler {
	return &OrdersHandler{repo: repo, calc: calc}
}

func (h *OrdersHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	role, _ := middleware.Role(r.Context())
	if role != "client" {
		jsonError(w, http.StatusForbidden, "only clients can create orders")
		return
	}
	var req CreateOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		observability.Stats().ObserveBusiness("order_create", "failure")
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateOrderRequest(req); err != nil {
		observability.Stats().ObserveBusiness("order_create", "failure")
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	distanceKm := distance.CalculateDistance(
		req.FromCoords.Latitude, req.FromCoords.Longitude,
		req.ToCoords.Latitude, req.ToCoords.Longitude,
	)
	price := h.calc.Calculate(distanceKm, req.Weight)
	order := models.NewOrder(userID, req.FromAddress, req.ToAddress, req.FromCoords, req.ToCoords, req.Weight, distanceKm, price)
	if err := h.repo.Create(r.Context(), order); err != nil {
		observability.Logger().Error("order_create_failed", "error", err, "user_id", userID)
		observability.Stats().ObserveBusiness("order_create", "failure")
		jsonError(w, http.StatusInternalServerError, "failed to create order")
		return
	}
	observability.Stats().ObserveBusiness("order_create", "success")
	jsonResponse(w, http.StatusCreated, map[string]any{
		"order":                      order,
		"estimated_duration_minutes": int(math.Ceil(distanceKm / 15 * 60)),
	})
}

func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := uuid.Parse(id); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	order, err := h.repo.GetByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get order")
		return
	}
	if order.UserID != userID {
		jsonError(w, http.StatusNotFound, "order not found")
		return
	}
	jsonResponse(w, http.StatusOK, order)
}

func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	role, ok := middleware.Role(r.Context())
	if !ok {
		jsonError(w, http.StatusForbidden, "role is required")
		return
	}

	page := parseBoundedInt(r.URL.Query().Get("page"), 1, 1, 1_000_000)
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 20, 1, 100)
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !validOrderStatus(status) {
		jsonError(w, http.StatusBadRequest, "invalid status")
		return
	}
	sort := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sort != "" && sort != "price_asc" && sort != "price_desc" {
		jsonError(w, http.StatusBadRequest, "invalid sort")
		return
	}

	var (
		orders []models.Order
		total  int
		err    error
	)
	switch role {
	case "client":
		orders, total, err = h.repo.ListByUser(r.Context(), userID, status, page, limit, sort)
	case "courier":
		orders, total, err = h.repo.ListForCourier(r.Context(), userID, status, page, limit, sort)
	default:
		jsonError(w, http.StatusForbidden, "unsupported role")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list orders")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"orders": orders, "total": total, "page": page, "limit": limit})
}

func validateOrderRequest(req CreateOrderRequest) error {
	if strings.TrimSpace(req.FromAddress.City) == "" || strings.TrimSpace(req.FromAddress.Street) == "" ||
		strings.TrimSpace(req.ToAddress.City) == "" || strings.TrimSpace(req.ToAddress.Street) == "" {
		return errors.New("from_address and to_address must contain city and street")
	}
	if req.Weight <= 0 || req.Weight > 100 || math.IsNaN(req.Weight) || math.IsInf(req.Weight, 0) {
		return errors.New("weight must be greater than 0 and at most 100 kg")
	}
	if !validCoordinates(req.FromCoords) || !validCoordinates(req.ToCoords) {
		return errors.New("invalid coordinates")
	}
	return nil
}

func validCoordinates(coords models.Coordinates) bool {
	return !math.IsNaN(coords.Latitude) && !math.IsNaN(coords.Longitude) &&
		!math.IsInf(coords.Latitude, 0) && !math.IsInf(coords.Longitude, 0) &&
		coords.Latitude >= -90 && coords.Latitude <= 90 && coords.Longitude >= -180 && coords.Longitude <= 180
}

func validOrderStatus(status string) bool {
	switch status {
	case models.StatusCreated, models.StatusAssigned, models.StatusAtPickup, models.StatusInProgress, models.StatusDelivered, models.StatusCancelled:
		return true
	default:
		return false
	}
}

func parseBoundedInt(raw string, fallback, minValue, maxValue int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minValue || value > maxValue {
		return fallback
	}
	return value
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

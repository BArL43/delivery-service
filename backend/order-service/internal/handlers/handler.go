package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"order-service/internal/models"
	"order-service/internal/observability"
	"order-service/internal/pricing"
	"order-service/internal/storage"
)

type CreateOrderRequest struct {
	FromAddress models.Address `json:"from_address"`
	ToAddress   models.Address `json:"to_address"`
	Weight      float64        `json:"weight"`
	DistanceKm  float64        `json:"distance_km"`
	UserID      string         `json:"user_id"`
}

type OrdersHandler struct {
	repo storage.OrderRepository
	calc *pricing.Calculator
}

func NewOrdersHandler(repo storage.OrderRepository, calc *pricing.Calculator) *OrdersHandler {
	return &OrdersHandler{
		repo: repo,
		calc: calc,
	}
}

func (h *OrdersHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		observability.Logger().Warn("order_create_decode_error", "error", err)
		observability.Stats().ObserveBusiness("order_create", "failure")
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Generate default user_id if not provided
	userID := req.UserID
	if userID == "" {
		userID = "00000000-0000-0000-0000-000000000000"
	}

	// Calculate price server-side based on weight and route distance
	price := h.calc.Calculate(req.DistanceKm, req.Weight)

	order := models.NewOrder(userID, req.FromAddress, req.ToAddress, req.Weight, price)
	if err := h.repo.Create(r.Context(), order); err != nil {
		observability.Logger().Error("order_create_failed", "error", err, "user_id", userID, "price", price)
		observability.Stats().ObserveBusiness("order_create", "failure")
		http.Error(w, `{"error": "failed to create order"}`, http.StatusInternalServerError)
		return
	}

	observability.Logger().Info("order_created",
		"order_id", order.ID,
		"user_id", userID,
		"price", price,
		"weight", req.Weight,
		"distance_km", req.DistanceKm,
	)
	observability.Stats().ObserveBusiness("order_create", "success")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

// GET /orders/{id} - Get one
func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	id = strings.TrimSuffix(id, "/")

	order, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// GET /orders/{id} - Get all
func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.repo.List(r.Context())
	if err != nil {
		http.Error(w, `{"error": "failed to list orders"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

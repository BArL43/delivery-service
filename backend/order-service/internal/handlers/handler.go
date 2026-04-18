package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"order-service/internal/models"
	"order-service/internal/storage"
)

type CreateOrderRequest struct {
	FromAddress models.Address `json:"from_address"`
	ToAddress   models.Address `json:"to_address"`
	Price       float64       `json:"price"`
	UserID      string        `json:"user_id"`
}

type OrdersHandler struct {
	repo storage.OrderRepository
}

func NewOrdersHandler(repo storage.OrderRepository) *OrdersHandler {
	return &OrdersHandler{
		repo: repo,
	}
}

func (h *OrdersHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	order := models.NewOrder(req.UserID, req.FromAddress, req.ToAddress, req.Price)
	if err := h.repo.Create(r.Context(), order); err != nil {
		http.Error(w, `{"error": "failed to create order"}`, http.StatusInternalServerError)
		return
	}

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

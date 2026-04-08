package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"api-gateway/internal/model"
)

type CreateOrderRequest struct {
	FromAddress model.Address `json:"from_address"`
	ToAddress   model.Address `json:"to_address"`
}

type OrdersHandler struct {
	orders map[string]model.Order
}

func (h *OrdersHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	json.NewDecoder(r.Body).Decode(&req)

	order := model.Order{
		ID:          "ORD-" + string(len(h.orders)+1),
		FromAddress: req.FromAddress,
		ToAddress:   req.ToAddress,
		Status:      "created",
	}
	h.orders[order.ID] = order

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// GET /orders/{id} - Get one
func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	id = strings.TrimSuffix(id, "/")

	order, ok := h.orders[id]
	if !ok {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// GET /orders/{id} - Get all
func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	var orders []model.Order
	for _, o := range h.orders {
		orders = append(orders, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

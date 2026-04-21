package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"order-service/internal/models"
	"order-service/internal/pricing"
	"order-service/internal/distance"
	"order-service/internal/storage"
	"strconv"
	"time"
)

type CreateOrderRequest struct {
	FromAddress models.Address     `json:"from_address"`
	ToAddress   models.Address     `json:"to_address"`
	FromCoords  models.Coordinates `json:"from_coords"`
	ToCoords    models.Coordinates `json:"to_coords"`
	Weight      float64            `json:"weight"`
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
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Weight <= 0 {
		http.Error(w, `{"error": "weight must be positive"}`, http.StatusBadRequest)
		return
	}
	// Calculate price server-side based on weight and route distance
	dist := distance.CalculateDistance(req.FromCoords.Latitude, req.FromCoords.Longitude, req.ToCoords.Latitude, req.ToCoords.Longitude)
	price := h.calc.Calculate(dist, req.Weight)

	order := models.NewOrder(userID, req.FromAddress, req.ToAddress, req.FromCoords, req.ToCoords, req.Weight, price)
	if err := h.repo.Create(r.Context(), order); err != nil {
		http.Error(w, `{"error": "failed to create order"}`, http.StatusInternalServerError)
		return
	}
	resp := models.OrderResponse{
		OrderId:           order.ID,
		InitialStatus:     order.Status,
		EstimatedDistance: distance,
		EstimatedDuration: time.Duration(distance / 15 * float64(time.Hour)),
		EstimatedPrice:    price,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GET /orders/{id} - Get one
func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userId, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	orderId := r.PathValue("id")
	if orderId == "" {
		http.Error(w, `{"error": "id is required"}`, http.StatusBadRequest)
		return
	}

	order, err := h.repo.GetByID(r.Context(), orderId, userId)
	if err != nil {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
		return
	}

	resp := models.OrderResponse{
		OrderId:           order.ID,
		InitialStatus:     order.Status,
		EstimatedDistance: pricing.CalculateDistance(order.FromCoords.Latitude, order.FromCoords.Longitude, order.ToCoords.Latitude, order.ToCoords.Longitude),
		EstimatedDuration: time.Duration(pricing.CalculateDistance(order.FromCoords.Latitude, order.FromCoords.Longitude, order.ToCoords.Latitude, order.ToCoords.Longitude) / 15 * float64(time.Hour)),
		EstimatedPrice:    order.Price,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GET /orders/{id} - Get all
func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	query := r.URL.Query()
	sort := query.Get("sort")
	status := query.Get("status")

	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	orders, total, err := h.repo.List(r.Context(), userID, status, page, limit, sort)
	if err != nil {
		http.Error(w, `{"error": "failed to list orders"}`, http.StatusInternalServerError)
		return
	}

	var items []models.OrderResponse
	for _, order := range orders {
		items = append(items, models.OrderResponse{
			OrderId:           order.ID,
			InitialStatus:     order.Status,
			EstimatedDistance: pricing.CalculateDistance(order.FromCoords.Latitude, order.FromCoords.Longitude, order.ToCoords.Latitude, order.ToCoords.Longitude),
			EstimatedDuration: time.Duration(pricing.CalculateDistance(order.FromCoords.Latitude, order.FromCoords.Longitude, order.ToCoords.Latitude, order.ToCoords.Longitude) / 15 * float64(time.Hour)),
			EstimatedPrice:    order.Price,
		})
	}
	resp := models.OrderListResponse{
		Orders: items,
		Total:  total,
		Page:   page,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *OrdersHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	orderID := r.PathValue("orderId")
	if orderID == "" {
		http.Error(w, `{"error": "missing order ID"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	currentStatus, err := h.repo.GetCurrentStatus(r.Context(), orderID, userID)
	if err != nil {
		http.Error(w, `{"error": "order not found"}`, http.StatusNotFound)
		return
	}

	if !models.IsValidTransition(currentStatus, req.NewStatus) {
		errMsg := fmt.Sprintf(`{"error": "invalid status transition from %s to %s"}`, currentStatus, req.NewStatus)
		http.Error(w, errMsg, http.StatusConflict)
		return
	}

	err = h.repo.UpdateStatus(r.Context(), orderID, userID, req.NewStatus, req.Reason)
	if err != nil {
		http.Error(w, `{"error": "failed to update status"}`, http.StatusInternalServerError)
		return
	}

	resp := models.UpdateOrderStatusResponse{
		UpdatedStatus: req.NewStatus,
		ChangedAt:     time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

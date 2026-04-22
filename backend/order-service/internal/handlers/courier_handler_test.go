package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"order-service/internal/models"

	"github.com/jackc/pgx/v5"
)

// --- Mock repositories ---

type mockCourierRepo struct {
	courier      *models.Courier
	orders       map[string]*models.Order
	available    []models.Courier
	updateErr    error
	locationErr  error
	getErr       error
	setActiveErr error
}

func (m *mockCourierRepo) Create(ctx context.Context, courier models.Courier) error { return nil }
func (m *mockCourierRepo) GetByID(ctx context.Context, id string) (*models.Courier, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.courier == nil {
		return nil, nil
	}
	c := *m.courier
	return &c, nil
}
func (m *mockCourierRepo) GetByEmail(ctx context.Context, email string) (*models.Courier, error) {
	return m.GetByID(ctx, "")
}
func (m *mockCourierRepo) GetByActiveOrderID(ctx context.Context, orderID string) (*models.Courier, error) {
	if m.courier == nil {
		return nil, pgx.ErrNoRows
	}
	c := *m.courier
	if c.ActiveOrderID == nil || *c.ActiveOrderID != orderID {
		return nil, pgx.ErrNoRows
	}
	return &c, nil
}
func (m *mockCourierRepo) UpdateStatus(ctx context.Context, id string, isOnline bool, transportType string) error {
	return m.updateErr
}
func (m *mockCourierRepo) UpdateLocation(ctx context.Context, id string, lat, lon float64) error {
	return m.locationErr
}
func (m *mockCourierRepo) GetActiveCourierOrder(ctx context.Context, courierID string) (*models.Order, error) {
	if o, ok := m.orders[courierID]; ok {
		c := *o
		return &c, nil
	}
	return nil, nil
}
func (m *mockCourierRepo) UnassignActiveOrder(ctx context.Context, courierID string) error {
	return nil
}
func (m *mockCourierRepo) FindAvailable(ctx context.Context) ([]models.Courier, error) {
	return m.available, nil
}
func (m *mockCourierRepo) SetActiveOrder(ctx context.Context, courierID, orderID string) error {
	return m.setActiveErr
}

type mockAssignmentRepo struct {
	createErr error
	byOrder   *models.Assignment
	byCourier *models.Assignment
	updateErr error
}

func (m *mockAssignmentRepo) Create(ctx context.Context, a models.Assignment) error {
	return m.createErr
}
func (m *mockAssignmentRepo) GetByOrderID(ctx context.Context, orderID string) (*models.Assignment, error) {
	if m.byOrder == nil {
		return nil, pgx.ErrNoRows
	}
	c := *m.byOrder
	return &c, nil
}
func (m *mockAssignmentRepo) GetActiveByCourierID(ctx context.Context, courierID string) (*models.Assignment, error) {
	if m.byCourier == nil {
		return nil, nil
	}
	c := *m.byCourier
	return &c, nil
}
func (m *mockAssignmentRepo) UpdateStatus(ctx context.Context, orderID string, newStatus string) error {
	return m.updateErr
}

type mockOrderRepo struct {
	updateErr error
}

func (m *mockOrderRepo) Create(ctx context.Context, order models.Order) error { return nil }
func (m *mockOrderRepo) GetByID(ctx context.Context, id string) (*models.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) List(ctx context.Context) ([]models.Order, error) { return nil, nil }
func (m *mockOrderRepo) UpdateStatus(ctx context.Context, orderID string, newStatus string) error {
	return m.updateErr
}

// --- Tests ---

func TestToggleAvailability(t *testing.T) {
	courier := &models.Courier{
		ID:            "test-id",
		TransportType: "bicycle",
	}
	h := NewCourierHandler(&mockCourierRepo{courier: courier}, nil, nil)

	body := `{"courier_id":"test-id","is_online":true,"transport_type":"car"}`
	req := httptest.NewRequest("POST", "/api/v1/couriers/availability", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ToggleAvailability(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["transport_type"] != "car" {
		t.Errorf("expected transport_type=car, got %v", resp["transport_type"])
	}
}

func TestToggleAvailability_MissingCourierID(t *testing.T) {
	h := NewCourierHandler(&mockCourierRepo{}, nil, nil)
	body := `{"is_online":true}`
	req := httptest.NewRequest("POST", "/api/v1/couriers/availability", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ToggleAvailability(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateLocation(t *testing.T) {
	h := NewCourierHandler(&mockCourierRepo{}, nil, nil)
	body := `{"courier_id":"test-id","lat":55.7,"lon":37.6}`
	req := httptest.NewRequest("POST", "/api/v1/couriers/location", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.UpdateLocation(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}
}

func TestUpdateLocation_MissingCourierID(t *testing.T) {
	h := NewCourierHandler(&mockCourierRepo{}, nil, nil)
	body := `{"lat":55.7,"lon":37.6}`
	req := httptest.NewRequest("POST", "/api/v1/couriers/location", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.UpdateLocation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAssignOrder_ManualMode(t *testing.T) {
	courier := &models.Courier{
		ID:            "courier-1",
		TransportType: "car",
	}
	h := NewCourierHandler(
		&mockCourierRepo{courier: courier},
		&mockAssignmentRepo{},
		&mockOrderRepo{},
	)
	body := `{"courier_id":"courier-1","mode":"manual"}`
	req := httptest.NewRequest("POST", "/orders/order-1/assign", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.AssignOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["courier_id"] != "courier-1" {
		t.Errorf("expected courier_id=courier-1, got %v", resp["courier_id"])
	}
}

func TestAssignOrder_AutoMode_NoAvailable(t *testing.T) {
	h := NewCourierHandler(
		&mockCourierRepo{available: []models.Courier{}},
		&mockAssignmentRepo{},
		&mockOrderRepo{},
	)
	body := `{"mode":"auto"}`
	req := httptest.NewRequest("POST", "/orders/order-1/assign", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.AssignOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAssignOrder_ManualMode_MissingCourierID(t *testing.T) {
	h := NewCourierHandler(&mockCourierRepo{}, nil, nil)
	body := `{"mode":"manual"}`
	req := httptest.NewRequest("POST", "/orders/order-1/assign", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.AssignOrder(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetActiveOrder(t *testing.T) {
	order := &models.Order{
		ID:          "order-1",
		FromAddress: models.Address{City: "Москва", Street: "Тверская"},
		ToAddress:   models.Address{City: "Москва", Street: "Арбат"},
	}
	h := NewCourierHandler(
		&mockCourierRepo{orders: map[string]*models.Order{"courier-1": order}},
		nil,
		nil,
	)
	req := httptest.NewRequest("GET", "/couriers/courier-1/active-order", nil)
	w := httptest.NewRecorder()

	h.GetActiveOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["order_id"] != "order-1" {
		t.Errorf("expected order_id=order-1, got %v", resp["order_id"])
	}
}

func TestUpdateOrderStatus_RecreatesMissingAssignment(t *testing.T) {
	orderID := "order-1"
	courierID := "courier-1"
	courier := &models.Courier{ID: courierID}
	courier.ActiveOrderID = &orderID

	h := NewCourierHandler(
		&mockCourierRepo{courier: courier},
		&mockAssignmentRepo{},
		&mockOrderRepo{},
	)
	body := `{"courier_id":"courier-1","status":"at_pickup"}`
	req := httptest.NewRequest("PATCH", "/api/v1/orders/order-1/status", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "at_pickup" {
		t.Fatalf("expected status=at_pickup, got %v", resp["status"])
	}
}

func TestComputeETA(t *testing.T) {
	tests := []struct {
		transport string
		distance  float64
		want      string
	}{
		{"bicycle", 15, "60m"},   // 15km / 15kmh * 60 = 60min
		{"bicycle", 7.5, "30m"},  // 7.5 / 15 * 60 = 30
		{"scooter", 12.5, "30m"}, // 12.5 / 25 * 60 = 30
		{"car", 40, "60m"},       // 40 / 40 * 60 = 60
		{"car", 20, "30m"},       // 20 / 40 * 60 = 30
		{"unknown", 15, "60m"},   // default bicycle speed
	}

	for _, tt := range tests {
		t.Run(tt.transport+"_"+tt.want, func(t *testing.T) {
			got := computeETA(tt.transport, tt.distance)
			if got != tt.want {
				t.Errorf("computeETA(%q, %v) = %q, want %q", tt.transport, tt.distance, got, tt.want)
			}
		})
	}
}

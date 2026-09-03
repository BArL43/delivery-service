package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"order-service/internal/middleware"
	"order-service/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mockCourierRepo struct {
	courier     *models.Courier
	orders      map[string]*models.Order
	available   []models.Courier
	updateErr   error
	locationErr error
	created     *models.Courier
}

func (m *mockCourierRepo) Create(_ context.Context, courier models.Courier) error {
	m.created = &courier
	return nil
}
func (m *mockCourierRepo) GetByID(_ context.Context, id string) (*models.Courier, error) {
	if m.courier == nil || (id != "" && m.courier.ID != id) {
		return nil, pgx.ErrNoRows
	}
	copy := *m.courier
	return &copy, nil
}
func (m *mockCourierRepo) GetByUserID(_ context.Context, userID string) (*models.Courier, error) {
	if m.courier == nil || m.courier.UserID != userID {
		return nil, pgx.ErrNoRows
	}
	copy := *m.courier
	return &copy, nil
}
func (m *mockCourierRepo) GetByEmail(_ context.Context, email string) (*models.Courier, error) {
	if m.courier == nil || m.courier.Email != email {
		return nil, pgx.ErrNoRows
	}
	copy := *m.courier
	return &copy, nil
}
func (m *mockCourierRepo) GetByActiveOrderID(_ context.Context, orderID string) (*models.Courier, error) {
	if m.courier == nil || m.courier.ActiveOrderID == nil || *m.courier.ActiveOrderID != orderID {
		return nil, pgx.ErrNoRows
	}
	copy := *m.courier
	return &copy, nil
}
func (m *mockCourierRepo) UpdateStatus(_ context.Context, _ string, _ bool, _ string) error {
	return m.updateErr
}
func (m *mockCourierRepo) UpdateLocation(_ context.Context, _ string, _, _ float64) error {
	return m.locationErr
}
func (m *mockCourierRepo) GetActiveCourierOrder(_ context.Context, courierID string) (*models.Order, error) {
	if order, ok := m.orders[courierID]; ok {
		copy := *order
		return &copy, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockCourierRepo) UnassignActiveOrder(_ context.Context, _ string) error { return nil }
func (m *mockCourierRepo) FindAvailable(_ context.Context) ([]models.Courier, error) {
	return m.available, nil
}
func (m *mockCourierRepo) SetActiveOrder(_ context.Context, _, _ string) error { return nil }

type mockAssignmentRepo struct {
	assignErr     error
	transitionErr error
	assigned      *models.Assignment
	transitioned  bool
}

func (m *mockAssignmentRepo) Create(_ context.Context, _ models.Assignment) error { return nil }
func (m *mockAssignmentRepo) Assign(_ context.Context, a models.Assignment) error {
	copy := a
	m.assigned = &copy
	return m.assignErr
}
func (m *mockAssignmentRepo) GetByOrderID(_ context.Context, _ string) (*models.Assignment, error) {
	return nil, pgx.ErrNoRows
}
func (m *mockAssignmentRepo) GetActiveByCourierID(_ context.Context, _ string) (*models.Assignment, error) {
	return nil, pgx.ErrNoRows
}
func (m *mockAssignmentRepo) UpdateStatus(_ context.Context, _, _ string) error { return nil }
func (m *mockAssignmentRepo) Transition(_ context.Context, _, _, _ string) error {
	m.transitioned = true
	return m.transitionErr
}

type mockOrderRepo struct {
	order *models.Order
}

func (m *mockOrderRepo) Create(_ context.Context, _ models.Order) error { return nil }
func (m *mockOrderRepo) GetByID(_ context.Context, id string) (*models.Order, error) {
	if m.order == nil || m.order.ID != id {
		return nil, pgx.ErrNoRows
	}
	copy := *m.order
	return &copy, nil
}
func (m *mockOrderRepo) List(_ context.Context) ([]models.Order, error)    { return nil, nil }
func (m *mockOrderRepo) UpdateStatus(_ context.Context, _, _ string) error { return nil }

func withUser(req *http.Request, userID string) *http.Request {
	return req.WithContext(middleware.WithIdentity(req.Context(), userID, "client"))
}

func TestRegisterCourierUsesAuthenticatedUser(t *testing.T) {
	repo := &mockCourierRepo{}
	h := NewCourierHandler(repo, nil, nil)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/register", strings.NewReader(`{"email":"courier@example.com","full_name":"Test Courier","phone":"+79990000000","transport_type":"bicycle"}`)), "42")
	w := httptest.NewRecorder()

	h.RegisterCourier(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if repo.created == nil || repo.created.UserID != "42" {
		t.Fatalf("expected courier to be bound to authenticated user")
	}
}

func TestToggleAvailabilityRejectsAnotherCourier(t *testing.T) {
	courier := &models.Courier{ID: uuid.NewString(), UserID: "42", TransportType: "bicycle"}
	h := NewCourierHandler(&mockCourierRepo{courier: courier}, nil, nil)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/availability", strings.NewReader(`{"courier_id":"another","is_online":true}`)), "42")
	w := httptest.NewRecorder()

	h.ToggleAvailability(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdateLocationValidatesCoordinates(t *testing.T) {
	courier := &models.Courier{ID: uuid.NewString(), UserID: "42", TransportType: "bicycle"}
	h := NewCourierHandler(&mockCourierRepo{courier: courier}, nil, nil)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/location", strings.NewReader(`{"lat":95,"lon":37.6}`)), "42")
	w := httptest.NewRecorder()

	h.UpdateLocation(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAssignOrderUsesTransactionalRepository(t *testing.T) {
	orderID := uuid.NewString()
	courierID := uuid.NewString()
	courier := &models.Courier{ID: courierID, UserID: "77", TransportType: "car", IsOnline: true}
	assignments := &mockAssignmentRepo{}
	h := NewCourierHandler(
		&mockCourierRepo{courier: courier},
		assignments,
		&mockOrderRepo{order: &models.Order{ID: orderID, UserID: "42", DistanceKm: 20}},
	)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/assign", strings.NewReader(`{"courier_id":"`+courierID+`","mode":"manual"}`)), "42")
	req.SetPathValue("orderId", orderID)
	w := httptest.NewRecorder()

	h.AssignOrder(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if assignments.assigned == nil || assignments.assigned.OrderID != orderID || assignments.assigned.CourierID != courierID {
		t.Fatalf("expected atomic assignment repository to be used")
	}
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["eta"] != "30m" {
		t.Fatalf("expected ETA from actual order distance, got %v", response["eta"])
	}
}

func TestAssignOrderDoesNotExposeAnotherUsersOrder(t *testing.T) {
	orderID := uuid.NewString()
	h := NewCourierHandler(
		&mockCourierRepo{},
		&mockAssignmentRepo{},
		&mockOrderRepo{order: &models.Order{ID: orderID, UserID: "other"}},
	)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/assign", strings.NewReader(`{"mode":"auto"}`)), "42")
	req.SetPathValue("orderId", orderID)
	w := httptest.NewRecorder()

	h.AssignOrder(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateOrderStatusUsesOwnedCourier(t *testing.T) {
	orderID := uuid.NewString()
	courier := &models.Courier{ID: uuid.NewString(), UserID: "42"}
	assignments := &mockAssignmentRepo{}
	h := NewCourierHandler(&mockCourierRepo{courier: courier}, assignments, nil)
	req := withUser(httptest.NewRequest(http.MethodPatch, "/api/v1/orders/"+orderID+"/status", strings.NewReader(`{"status":"at_pickup"}`)), "42")
	req.SetPathValue("orderId", orderID)
	w := httptest.NewRecorder()

	h.UpdateOrderStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !assignments.transitioned {
		t.Fatal("expected transactional status transition")
	}
}

func TestComputeETA(t *testing.T) {
	tests := []struct {
		transport string
		distance  float64
		want      string
	}{
		{"bicycle", 15, "60m"},
		{"scooter", 12.5, "30m"},
		{"car", 20, "30m"},
		{"unknown", 15, "60m"},
	}
	for _, tt := range tests {
		if got := computeETA(tt.transport, tt.distance); got != tt.want {
			t.Fatalf("computeETA(%q, %v) = %q, want %q", tt.transport, tt.distance, got, tt.want)
		}
	}
}

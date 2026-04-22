package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"order-service/internal/models"
	"order-service/internal/storage"
)

// --- 1. МОК РЕПОЗИТОРИЯ ---
type mockOrderRepo struct {
	storage.OrderRepository
	getErr error
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id, userID string) (*models.Order, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &models.Order{ID: id, Status: "CREATED", Price: 500}, nil
}

// --- 2. ТЕСТИРОВАНИЕ СОЗДАНИЯ ЗАКАЗА (ВАЛИДАЦИЯ) ---

func TestOrdersHandler_CreateOrder_Validation(t *testing.T) {
	handler := NewOrdersHandler(nil, nil)

	tests := []struct {
		name           string
		body           string
		withAuth       bool
		expectedStatus int
	}{
		{
			name:           "Нет авторизации (нет userID)",
			body:           `{"weight": 10.5}`,
			withAuth:       false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Отрицательный вес",
			body:           `{"weight": -5.0}`,
			withAuth:       true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Сломанный JSON",
			body:           `{"weight": oops, "from_address": }`,
			withAuth:       true,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(tc.body))

			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "userID", "user-123")
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.CreateOrder(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("Ожидался статус %d, получено %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}

// --- 3. ТЕСТИРОВАНИЕ ПОЛУЧЕНИЯ ЗАКАЗА (РАБОТА С БД) ---

func TestOrdersHandler_GetOrder_NotFound(t *testing.T) {
	mockRepo := &mockOrderRepo{
		getErr: errors.New("order not found in database"),
	}

	handler := NewOrdersHandler(mockRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders/999", nil)

	ctx := context.WithValue(req.Context(), "userID", "user-123")
	req = req.WithContext(ctx)

	req.SetPathValue("id", "999")

	rr := httptest.NewRecorder()
	handler.GetOrder(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 404 Not Found, получено %d", rr.Code)
	}
}

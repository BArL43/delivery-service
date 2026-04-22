package geocoder

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- 1. ПИШЕМ МОКИ (ЗАГЛУШКИ) ДЛЯ ИНТЕРФЕЙСОВ ---

type mockCache struct {
	coords *Coordinates
	err    error
}

func (m *mockCache) Get(ctx context.Context, key string) (*Coordinates, error) {
	return m.coords, m.err
}
func (m *mockCache) Set(ctx context.Context, key string, coords *Coordinates) error {
	return nil
}

type mockProvider struct {
	name       string
	coords     *Coordinates
	geocodeErr error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	return m.coords, m.geocodeErr
}
func (m *mockProvider) Suggest(ctx context.Context, query string, limit int) ([]AddressSuggestion, error) {
	return nil, nil
}
func (m *mockProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*ReversResponse, error) {
	return nil, nil
}

// --- 2. ТЕСТИРУЕМ БИЗНЕС-ЛОГИКУ СЕРВИСА (FALLBACK) ---

func TestService_GetCoordinates_Fallback(t *testing.T) {
	expectedCoords := &Coordinates{Latitude: 55.75, Longitude: 37.61}

	emptyCache := &mockCache{coords: nil, err: errors.New("not found")}
	primaryFailing := &mockProvider{name: "PrimaryAPI", geocodeErr: errors.New("api timeout")}
	fallbackSuccess := &mockProvider{name: "FallbackAPI", coords: expectedCoords, geocodeErr: nil}

	svc := NewService(emptyCache, primaryFailing, fallbackSuccess)

	coords, providerName, err := svc.GetCoordinates(context.Background(), "Москва, Красная Площадь")

	if err != nil {
		t.Fatalf("Ожидался успех, получена ошибка: %v", err)
	}

	if providerName != "FallbackAPI" {
		t.Errorf("Ожидалось, что отработает резервный провайдер 'FallbackAPI', а отработал '%s'", providerName)
	}

	if coords.Latitude != expectedCoords.Latitude {
		t.Errorf("Координаты не совпадают")
	}
}

// --- 3. ТЕСТИРУЕМ ВАЛИДАЦИЮ ХЕНДЛЕРА ---

func TestGeocodeHandler_ReverseGeocode_Validation(t *testing.T) {
	handler := NewGeocodeHandler(nil)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "Невалидный JSON",
			body:           `{"lat": 55.75, "lon": oops}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Широта больше 90 (ошибка)",
			body:           `{"lat": 95.0, "lon": 37.61}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Долгота меньше -180 (ошибка)",
			body:           `{"lat": 55.75, "lon": -200.0}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/reverse", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			handler.ReverseGeocode(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("Ожидался статус %d, получено %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}

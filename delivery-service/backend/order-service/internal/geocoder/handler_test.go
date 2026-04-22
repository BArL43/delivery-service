package geocoder

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeocodeHandler_Validation(t *testing.T) {
	handler := NewGeocodeHandler(nil)

	tests := []struct {
		name           string
		method         string
		url            string
		handlerFunc    http.HandlerFunc
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Geocode: Пустой адрес",
			method:         http.MethodGet,
			url:            "/geocode?address=%20%20%20",
			handlerFunc:    handler.GeocodeAddress,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error": "address query parameter is required"}`,
		},
		{
			name:           "Geocode: Нет параметра address",
			method:         http.MethodGet,
			url:            "/geocode?city=Moscow",
			handlerFunc:    handler.GeocodeAddress,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error": "address query parameter is required"}`,
		},
		{
			name:           "Suggest: Пустой запрос",
			method:         http.MethodGet,
			url:            "/suggest",
			handlerFunc:    handler.Suggest,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error": "query parameter is required"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, nil)

			rr := httptest.NewRecorder()

			tc.handlerFunc(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("Ожидался статус %d, получено %d", tc.expectedStatus, rr.Code)
			}

			actualBody := strings.TrimSpace(rr.Body.String())
			if actualBody != tc.expectedBody {
				t.Errorf("Ожидалось тело ответа '%s', получено '%s'", tc.expectedBody, actualBody)
			}
		})
	}
}

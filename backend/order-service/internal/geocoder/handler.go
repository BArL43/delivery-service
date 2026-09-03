package geocoder

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type GeocodeHandler struct {
	service *Service
}

func NewGeocodeHandler(service *Service) *GeocodeHandler {
	return &GeocodeHandler{service: service}
}

type GeocodeResponse struct {
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"lon"`
	NormalizedAddress string  `json:"normalizedAddress"`
	Provider          string  `json:"provider"`
}

func (h *GeocodeHandler) GeocodeAddress(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		writeError(w, http.StatusBadRequest, "address query parameter is required")
		return
	}
	parts := []string{address}
	if city := strings.TrimSpace(r.URL.Query().Get("city")); city != "" {
		parts = append(parts, city)
	}
	if country := strings.TrimSpace(r.URL.Query().Get("country")); country != "" {
		parts = append(parts, country)
	}
	fullAddress := strings.Join(parts, ", ")
	coords, provider, err := h.service.GetCoordinates(r.Context(), fullAddress)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to geocode address")
		return
	}
	writeJSON(w, http.StatusOK, GeocodeResponse{
		Lat: coords.Latitude, Lon: coords.Longitude,
		NormalizedAddress: NormalizeAddress(fullAddress), Provider: provider,
	})
}

func (h *GeocodeHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if len([]rune(query)) < 3 {
		writeError(w, http.StatusBadRequest, "query must contain at least 3 characters")
		return
	}
	limit := 5
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 20 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 20")
			return
		}
		limit = value
	}
	suggestions, err := h.service.SuggestAddress(r.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to get suggestions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": suggestions})
}

type reverseGeocodeRequest struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func (h *GeocodeHandler) ReverseGeocode(w http.ResponseWriter, r *http.Request) {
	var req reverseGeocodeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain exactly one JSON object")
		return
	}
	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		writeError(w, http.StatusBadRequest, "invalid latitude or longitude")
		return
	}
	result, err := h.service.ReverseAddress(r.Context(), req.Lat, req.Lon)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reverse geocode coordinates")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

package geocoder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type GeocodeHandler struct {
	service *Service
}

func NewGeocodeHandler(service *Service) *GeocodeHandler {
	return &GeocodeHandler{
		service: service,
	}
}

type GeocodeResponse struct {
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"lon"`
	NormalizedAddress string  `json:"normalizedAddress"`
	Provider          string  `json:"provider"`
}

func (h *GeocodeHandler) GeocodeAddress(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	address := query.Get("address")
	city := query.Get("city")
	country := query.Get("country")

	if strings.TrimSpace(address) == "" {
		http.Error(w, `{"error": "address query parameter is required"}`, http.StatusBadRequest)
		return
	}

	var addressParts []string
	addressParts = append(addressParts, address)
	if city != "" {
		addressParts = append(addressParts, city)
	}
	if country != "" {
		addressParts = append(addressParts, country)
	}

	fullAddress := strings.Join(addressParts, ", ")

	coords, provider, err := h.service.GetCoordinates(r.Context(), fullAddress)
	if err != nil {
		http.Error(w, `{"error": "failed to geocode address"}`, http.StatusInternalServerError)
		return
	}

	resp := GeocodeResponse{
		Lat:               coords.Latitude,
		Lon:               coords.Longitude,
		NormalizedAddress: NormalizeAddress(fullAddress),
		Provider:          provider,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *GeocodeHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := query.Get("query")

	if strings.TrimSpace(q) == "" {
		http.Error(w, `{"error": "query parameter is required"}`, http.StatusBadRequest)
		return
	}

	limitStr := query.Get("limit")
	limit := 5
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	if limit > 20 {
		limit = 20
	}

	suggestions, err := h.service.SuggestAddress(r.Context(), q, limit)
	if err != nil {
		http.Error(w, `{"error": "failed to get suggestions"}`, http.StatusInternalServerError)
		return
	}

	type suggestResponse struct {
		Suggestions []AddressSuggestion `json:"suggestions"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(suggestResponse{
		Suggestions: suggestions,
	})
}
